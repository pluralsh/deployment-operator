package cachewatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type taskFunc func()

// taskManager manages a set of tasks with object identifiers.
// This makes starting and stopping the tasks thread-safe.
type taskManager struct {
	lock        sync.Mutex
	cancelFuncs map[object.ObjMetadata]context.CancelFunc
}

func (tm *taskManager) Schedule(parentCtx context.Context, id object.ObjMetadata, delay time.Duration, task taskFunc) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if tm.cancelFuncs == nil {
		tm.cancelFuncs = make(map[object.ObjMetadata]context.CancelFunc)
	}

	cancel, found := tm.cancelFuncs[id]
	if found {
		// Cancel the existing scheduled task and replace it.
		cancel()
	}

	taskCtx, cancel := context.WithTimeout(context.Background(), delay)
	tm.cancelFuncs[id] = cancel

	go func() {
		log.Logger.Info("Task scheduled (%v) for object (%s)", delay, id)
		select {
		case <-parentCtx.Done():
			// stop waiting
			cancel()
		case <-taskCtx.Done():
			if taskCtx.Err() == context.DeadlineExceeded {
				log.Logger.Info("Task executing (after %v) for object (%v)", delay, id)
				task()
			}
			// else stop waiting
		}
	}()
}

func (tm *taskManager) Cancel(id object.ObjMetadata) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	cancelFunc, found := tm.cancelFuncs[id]
	if !found {
		// already cancelled or not added
		return
	}
	delete(tm.cancelFuncs, id)
	cancelFunc()
	if len(tm.cancelFuncs) == 0 {
		tm.cancelFuncs = nil
	}
}

// GroupKindNamespace identifies an informer target.
// When used as an informer target, the namespace is optional.
// When the namespace is empty for namespaced resources, all namespaces are watched.
type GroupKindNamespace struct {
	Group     string
	Kind      string
	Namespace string
}

type ObjectStatusReporter struct {
	// StatusReader specifies a custom implementation of the
	// engine.StatusReader interface that will be used to compute reconcile
	// status for resource objects.
	StatusReader engine.StatusReader

	// ObjectFilter is used to decide which objects to ingore.
	ObjectFilter ObjectFilter

	// ClusterReader is used to look up generated objects on-demand.
	// Generated objects (ex: Deployment > ReplicaSet > Pod) are sometimes
	// required for computing parent object status, to compensate for
	// controllers that aren't following status conventions.
	ClusterReader engine.ClusterReader

	// Mapper is used to map from GroupKind to GroupVersionKind.
	Mapper meta.RESTMapper

	LabelSelector labels.Selector

	// DynamicClient is used to watch of resource objects.
	DynamicClient dynamic.Interface

	// GroupKinds is the list of GroupKinds to watch.
	Targets []GroupKindNamespace

	// lock guards modification of the subsequent stateful fields
	lock sync.Mutex
	// context will be cancelled when the reporter should stop.
	context context.Context

	// cancel function that stops the context.
	// This should only be called after the terminal error event has been sent.
	cancel context.CancelFunc

	// funnel multiplexes multiple input channels into one output channel,
	// allowing input channels to be added and removed at runtime.
	funnel *eventFunnel

	// taskManager makes it possible to cancel scheduled tasks.
	taskManager *taskManager

	started bool
	stopped bool
}

func (w *ObjectStatusReporter) Start(ctx context.Context) <-chan event.Event {
	w.lock.Lock()
	defer w.lock.Unlock()

	if w.started {
		panic("ObjectStatusInformer cannot be restarted")
	}

	w.taskManager = &taskManager{}

	ctx, cancel := context.WithCancel(ctx)
	w.context = ctx
	w.cancel = cancel

	w.funnel = newEventFunnel(ctx)

	// Send start requests.
	for _, gkn := range w.Targets {
		go w.startWatcher(gkn)
	}

	w.started = true

	// Block until the event funnel is closed.
	// The event funnel will close after all the informer channels are closed.
	// The informer channels will close after the informers have stopped.
	// The informers will stop after their context is cancelled.
	go func() {
		<-w.funnel.Done()

		w.lock.Lock()
		defer w.lock.Unlock()
		w.stopped = true
	}()

	return w.funnel.OutputChannel()
}

func (w *ObjectStatusReporter) startWatcher(gkn GroupKindNamespace) {
	gk := schema.GroupKind{Group: gkn.Group, Kind: gkn.Kind}
	mapping, err := w.Mapper.RESTMapping(gk)
	gvr := gvrFromGvk(mapping.GroupVersionKind)
	watch, err := w.DynamicClient.Resource(gvr).Watch(w.context, metav1.ListOptions{
		LabelSelector: w.LabelSelector.String(),
	})
	if err != nil {
		log.Logger.Errorf("unexpected error establishing watch: %v\n", err)
		return
	}
	eventCh := make(chan event.Event)
	// Add this event channel to the output multiplexer
	w.funnel.AddInputChannel(eventCh)
	// Send SyncEvent immediately.
	eventCh <- event.Event{Type: event.SyncEvent}

	go w.Reconcile(watch.ResultChan(), eventCh)
	go func() {
		defer close(eventCh)
		doneCh := w.context.Done()
		<-doneCh
		watch.Stop()
	}()
}

func (w *ObjectStatusReporter) Reconcile(echan <-chan watch.Event, eventCh chan event.Event) {
	for e := range echan {
		switch e.Type {
		case watch.Added:
			fallthrough
		case watch.Modified:
			un, err := common.ToUnstructured(e.Object)
			if err != nil {
				eventCh <- event.Event{
					Type:  event.ErrorEvent,
					Error: err,
				}
				return
			}
			if w.ObjectFilter != nil && w.ObjectFilter.Filter(un) {
				return
			}
			rs, err := w.readStatusFromObject(w.context, un)
			if err != nil {
				// Send error event and stop the reporter!
				eventCh <- event.Event{
					Type:  event.ErrorEvent,
					Error: fmt.Errorf("failed to compute object status: %w", err),
				}
				return
			}
			eventCh <- event.Event{
				Type:     event.ResourceUpdateEvent,
				Resource: rs,
			}
		case watch.Deleted:
			un, err := common.ToUnstructured(e.Object)
			if err != nil {
				eventCh <- event.Event{
					Type:  event.ErrorEvent,
					Error: err,
				}
				return
			}
			if w.ObjectFilter != nil && w.ObjectFilter.Filter(un) {
				return
			}

			eventCh <- event.Event{
				Type:     event.ResourceUpdateEvent,
				Resource: deletedStatus(cache.ResourceKeyFromUnstructured(un).ObjMetadata()),
			}
		case watch.Error:
			/*			eventCh <- event.Event{
						Type: event.ErrorEvent,
					}*/
		default:
			fmt.Printf("unexpected watch event: %#v\n", e)
		}
	}
}

// readStatusFromObject is a convenience function to read object status with a
// StatusReader using a ClusterReader to retrieve generated objects.
func (w *ObjectStatusReporter) readStatusFromObject(
	ctx context.Context,
	obj *unstructured.Unstructured,
) (*event.ResourceStatus, error) {
	return w.StatusReader.ReadStatusForObject(ctx, w.ClusterReader, obj)
}

func deletedStatus(id object.ObjMetadata) *event.ResourceStatus {
	// Status is always NotFound after deltion.
	// Passed obj represents the last known state, not the current state.
	result := &event.ResourceStatus{
		Identifier: id,
		Status:     status.NotFoundStatus,
		Message:    "Resource not found",
	}

	return &event.ResourceStatus{
		Identifier: id,
		Resource:   nil, // deleted object has no
		Status:     result.Status,
		Message:    result.Message,
		// If deleted with foreground deletion, a finalizer will have blocked
		// deletion until all the generated resources are deleted.
		// TODO: Handle lookup of generated resources when not using foreground deletion.
		GeneratedResources: nil,
	}
}

func (w *ObjectStatusReporter) handleFatalError(eventCh chan<- event.Event, err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	eventCh <- event.Event{
		Type:  event.ErrorEvent,
		Error: err,
	}
	w.Stop()
}

// Stop triggers the cancellation of the reporter context, and closure of the
// event channel without sending an error event.
func (w *ObjectStatusReporter) Stop() {
	w.cancel()
}

// newStatusCheckTaskFunc returns a taskFund that reads the status of an object
// from the cluster and sends it over the event channel.
//
// This method should only be used for generated resource objects, as it's much
// slower at scale than watching the resource for updates.
func (w *ObjectStatusReporter) newStatusCheckTaskFunc(
	ctx context.Context,
	eventCh chan<- event.Event,
	id object.ObjMetadata,
) taskFunc {
	return func() {
		// check again
		rs, err := w.readStatusFromCluster(ctx, id)
		if err != nil {
			// Send error event and stop the reporter!
			// TODO: retry N times before terminating
			w.handleFatalError(eventCh, err)
			return
		}
		eventCh <- event.Event{
			Type:     event.ResourceUpdateEvent,
			Resource: rs,
		}
	}
}

// readStatusFromCluster is a convenience function to read object status with a
// StatusReader using a ClusterReader to retrieve the object and its generated
// objects.
func (w *ObjectStatusReporter) readStatusFromCluster(
	ctx context.Context,
	id object.ObjMetadata,
) (*event.ResourceStatus, error) {
	return w.StatusReader.ReadStatus(ctx, w.ClusterReader, id)
}
