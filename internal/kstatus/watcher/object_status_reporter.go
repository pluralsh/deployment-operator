// Copyright 2022 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/internal/kubernetes/watcher"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

// GroupKindNamespace identifies an informer target.
// When used as an informer target, the namespace is optional.
// When the namespace is empty for namespaced resources, all namespaces are watched.
type GroupKindNamespace struct {
	Group     string
	Kind      string
	Namespace string
}

// String returns a serialized form suitable for logging.
func (gkn GroupKindNamespace) String() string {
	return fmt.Sprintf("%s/%s/namespaces/%s",
		gkn.Group, gkn.Kind, gkn.Namespace)
}

func (gkn GroupKindNamespace) GroupKind() schema.GroupKind {
	return schema.GroupKind{Group: gkn.Group, Kind: gkn.Kind}
}

// ObjectStatusReporter reports on updates to objects (instances) using a
// network of informers to watch one or more resources (types).
//
// Unlike SharedIndexInformer, ObjectStatusReporter...
//   - Reports object status.
//   - Can watch multiple resource types simultaneously.
//   - Specific objects can be ignored for efficiency by specifying an ObjectFilter.
//   - Resolves GroupKinds into Resources at runtime, to pick up newly added
//     resources.
//   - Starts and Stops individual watches automaically to reduce errors when a
//     CRD or Namespace is deleted.
//   - Resources can be watched in root-scope mode or namespace-scope mode,
//     allowing the caller to optimize for efficiency or least-privilege.
//   - Gives unschedulable Pods (and objects that generate them) a 15s grace
//     period before reporting them as Failed.
//   - Resets the RESTMapper cache automatically when CRDs are modified.
//
// ObjectStatusReporter is NOT repeatable. It will panic if started more than
// once. If you need a repeatable factory, use DefaultStatusWatcher.
//
// Ref: https://github.com/kubernetes-sigs/cli-utils/blob/v0.37.1/pkg/kstatus/watcher/object_status_reporter.go
type ObjectStatusReporter struct {
	// Mapper is used to map from GroupKind to GroupVersionKind.
	Mapper meta.RESTMapper

	// StatusReader specifies a custom implementation of the
	// engine.StatusReader interface that will be used to compute reconcile
	// status for resource objects.
	StatusReader engine.StatusReader

	// ClusterReader is used to look up generated objects on-demand.
	// Generated objects (ex: Deployment > ReplicaSet > Pod) are sometimes
	// required for computing parent object status, to compensate for
	// controllers that aren't following status conventions.
	ClusterReader engine.ClusterReader

	// GroupKinds is the list of GroupKinds to watch.
	Targets []GroupKindNamespace

	// ObjectFilter is used to decide which objects to ignore.
	ObjectFilter kwatcher.ObjectFilter

	// RESTScope specifies whether to ListAndWatch resources at the namespace
	// or cluster (root) level. Using root scope is more efficient, but
	// namespace scope may require fewer permissions.
	RESTScope meta.RESTScope

	// DynamicClient is used to watch of resource objects.
	DynamicClient dynamic.Interface

	// DiscoveryClient is used to ensure if CRD exists on the server.
	DiscoveryClient discovery.CachedDiscoveryInterface

	// LabelSelector is used to apply server-side filtering on watched resources.
	LabelSelector labels.Selector

	// lock guards modification of the subsequent stateful fields
	lock sync.Mutex

	// gk2gkn maps GKs to GKNs to make it easy/cheap to look up.
	gk2gkn map[schema.GroupKind]map[GroupKindNamespace]struct{}

	// ns2gkn maps Namespaces to GKNs to make it easy/cheap to look up.
	ns2gkn map[string]map[GroupKindNamespace]struct{}

	// watcherRefs tracks which informers have been started and stopped
	watcherRefs map[GroupKindNamespace]*watcherReference

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

	id string
}

func (in *ObjectStatusReporter) Start(ctx context.Context) <-chan event.Event {
	in.lock.Lock()
	defer in.lock.Unlock()

	if in.started {
		panic("ObjectStatusInformer cannot be restarted")
	}

	in.taskManager = &taskManager{}

	// Map GroupKinds to sets of GroupKindNamespaces for fast lookups.
	// This is the only time we modify the map.
	// So it should be safe to read from multiple threads after this.
	in.gk2gkn = make(map[schema.GroupKind]map[GroupKindNamespace]struct{})
	for _, gkn := range in.Targets {
		gk := gkn.GroupKind()
		m, found := in.gk2gkn[gk]
		if !found {
			m = make(map[GroupKindNamespace]struct{})
			in.gk2gkn[gk] = m
		}
		m[gkn] = struct{}{}
	}

	// Map namespaces to sets of GroupKindNamespaces for fast lookups.
	// This is the only time we modify the map.
	// So it should be safe to read from multiple threads after this.
	in.ns2gkn = make(map[string]map[GroupKindNamespace]struct{})
	for _, gkn := range in.Targets {
		ns := gkn.Namespace
		m, found := in.ns2gkn[ns]
		if !found {
			m = make(map[GroupKindNamespace]struct{})
			in.ns2gkn[ns] = m
		}
		m[gkn] = struct{}{}
	}

	// Initialize the informer map with references to track their start/stop.
	// This is the only time we modify the map.
	// So it should be safe to read from multiple threads after this.
	if in.watcherRefs == nil {
		in.watcherRefs = make(map[GroupKindNamespace]*watcherReference)
	}

	for _, gkn := range in.Targets {
		if _, exists := in.watcherRefs[gkn]; !exists {
			in.watcherRefs[gkn] = &watcherReference{}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	in.context = ctx
	in.cancel = cancel

	// Use an event funnel to multiplex events through multiple input channels
	// into out output channel. We can't use the normal fan-in pattern, because
	// we need to be able to add and remove new input channels at runtime, as
	// new informers are created and destroyed.
	in.funnel = newEventFunnel(ctx)

	// Send start requests.
	for _, gkn := range in.Targets {
		in.startInformer(gkn)
	}

	in.started = true

	// Block until the event funnel is closed.
	// The event funnel will close after all the informer channels are closed.
	// The informer channels will close after the informers have stopped.
	// The informers will stop after their context is cancelled.
	go func() {
		<-in.funnel.Done()

		in.lock.Lock()
		defer in.lock.Unlock()
		in.stopped = true
	}()

	// Wait until all informers are synced or stopped, then send a SyncEvent.
	syncEventCh := make(chan event.Event)
	err := in.funnel.AddInputChannel(syncEventCh)
	if err != nil {
		// Reporter already stopped.
		return handleFatalError(fmt.Errorf("reporter failed to start: %w", err))
	}
	go func() {
		defer close(syncEventCh)
		// TODO: should we use something less aggressive, like wait.BackoffUntil?
		if cache.WaitForCacheSync(ctx.Done(), in.HasSynced) {
			syncEventCh <- event.Event{
				Type: event.SyncEvent,
			}
		}
	}()

	//nolint
	go wait.PollUntilContextCancel(ctx, 5*time.Second, false, func(_ context.Context) (done bool, err error) {
		stopped := true
		for _, ref := range in.watcherRefs {
			if ref.started {
				stopped = false
			}
		}

		if stopped {
			in.Stop()
		}

		return stopped, nil
	})

	return in.funnel.OutputChannel()
}

// Stop triggers the cancellation of the reporter context, and closure of the
// event channel without sending an error event.
func (in *ObjectStatusReporter) Stop() {
	klog.V(4).Info("Stopping reporter")
	in.cancel()
}

// HasSynced returns true if all the started informers have been synced.
//
// Use the following to block waiting for synchronization:
// synced := cache.WaitForCacheSync(stopCh, informer.HasSynced)
func (in *ObjectStatusReporter) HasSynced() bool {
	in.lock.Lock()
	defer in.lock.Unlock()

	if in.stopped || !in.started {
		return false
	}

	pending := make([]GroupKindNamespace, 0, len(in.watcherRefs))
	for gke, informer := range in.watcherRefs {
		if informer.HasStarted() && !informer.HasSynced() {
			pending = append(pending, gke)
		}
	}
	if len(pending) > 0 {
		klog.V(5).Infof("Informers pending synchronization: %v", pending)
		return false
	}
	return true
}

// startInformer adds the specified GroupKindNamespace to the start channel to
// be started asynchronously.
func (in *ObjectStatusReporter) startInformer(gkn GroupKindNamespace) {
	ctx, ok := in.watcherRefs[gkn].Start(in.context)
	if !ok {
		klog.V(5).Infof("Watch start skipped (already started): %v", gkn)
		// already started
		return
	}
	go in.startInformerWithRetry(ctx, gkn)
}

// stopInformer stops the informer watching the specified GroupKindNamespace.
func (in *ObjectStatusReporter) stopInformer(gkn GroupKindNamespace) {
	in.watcherRefs[gkn].Stop()
}

// restartInformer restarts the informer watching the specified GroupKindNamespace.
func (in *ObjectStatusReporter) restartInformer(gkn GroupKindNamespace) {
	in.watcherRefs[gkn].Restart()
}

func (in *ObjectStatusReporter) startInformerWithRetry(ctx context.Context, gkn GroupKindNamespace) {
	realClock := &clock.RealClock{}
	// TODO nolint can be removed once https://github.com/kubernetes/kubernetes/issues/118638 is resolved
	backoffManager := wait.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock) //nolint:staticcheck
	retryCtx, retryCancel := context.WithCancel(ctx)
	retryCount := 0

	wait.BackoffUntil(func() {
		err := in.startInformerNow(
			ctx,
			gkn,
		)
		if retryCount >= 2 {
			klog.V(3).Infof("Watch start abort, reached retry limit: %d: %v", retryCount, gkn)
			in.stopInformer(gkn)
			return
		}

		if err != nil {
			if meta.IsNoMatchError(err) {
				// CRD (or api extension) not installed, retry
				klog.V(3).Infof("Watch start error (blocking until CRD is added): %v: %v", gkn, err)
				in.restartInformer(gkn)
				retryCount++
				return
			}

			// Create a temporary input channel to send the error event.
			eventCh := make(chan event.Event)
			defer close(eventCh)
			funnelErr := in.funnel.AddInputChannel(eventCh)
			if funnelErr != nil {
				// Reporter already stopped.
				// This is fine. 🔥
				klog.V(5).Infof("Informer failed to start: %v", err)
				return
			}
			// Send error event and stop the reporter!
			in.handleFatalError(eventCh, err)
			return
		}
		// Success! - Stop retrying
		retryCancel()
	}, backoffManager, true, retryCtx.Done())
}

func (in *ObjectStatusReporter) newWatcher(ctx context.Context, gkn GroupKindNamespace) (watch.Interface, error) {
	gk := schema.GroupKind{Group: gkn.Group, Kind: gkn.Kind}
	mapping, err := in.Mapper.RESTMapping(gk)
	if err != nil {
		return nil, err
	}

	gvr := mapping.Resource

	var labelSelectorString string
	if in.LabelSelector != nil {
		labelSelectorString = in.LabelSelector.String()
	}

	w, err := watcher.NewRetryListerWatcher(
		watcher.WithListWatchFunc(
			func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labelSelectorString
				return in.DynamicClient.Resource(gvr).List(ctx, options)
			}, func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labelSelectorString
				return in.DynamicClient.Resource(gvr).Watch(ctx, options)
			}),
		watcher.WithID(gkn.String()),
	)
	if apierrors.IsNotFound(err) && !in.hasGVR(gvr) {
		return nil, &meta.NoKindMatchError{
			GroupKind:        gk,
			SearchedVersions: []string{gvr.Version},
		}
	}

	return w, err
}

func (in *ObjectStatusReporter) hasGVR(gvr schema.GroupVersionResource) bool {
	// Ensure we get fresh information about server resources
	in.DiscoveryClient.Invalidate()

	list, err := in.DiscoveryClient.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		klog.ErrorS(err, "failed to get discovery server resources", "gvr", gvr)
		return false
	}

	for _, resource := range list.APIResources {
		if gvr.Resource == resource.Name {
			return true
		}
	}

	return false
}

// startInformerNow starts an informer to watch for changes to a
// GroupKindNamespace. Changes are filtered and passed by event channel into the
// funnel. Each update event includes the computed status of the object.
// An error is returned if the informer could not be created.
func (in *ObjectStatusReporter) startInformerNow(
	ctx context.Context,
	gkn GroupKindNamespace,
) error {
	w, err := in.newWatcher(ctx, gkn)
	if err != nil {
		return err
	}

	in.watcherRefs[gkn].SetInformer(w)
	eventCh := make(chan event.Event)

	// Add this event channel to the output multiplexer
	err = in.funnel.AddInputChannel(eventCh)
	if err != nil {
		// Reporter already stopped.
		return fmt.Errorf("informer failed to build event handler: %w\n", err)
	}

	// Start the informer in the background.
	// Informer will be stopped when the context is cancelled.
	go func() {
		klog.V(3).Infof("Watch starting: %v", gkn)
		metrics.Record().ResourceCacheWatchStart(gkn.String())
		in.Run(ctx.Done(), w.ResultChan(), in.eventHandler(ctx, eventCh))
		metrics.Record().ResourceCacheWatchEnd(gkn.String())
		klog.V(3).Infof("Watch stopped: %v", gkn)
		// Signal to the caller there will be no more events for this GroupKind.
		in.watcherRefs[gkn].Stop()
		close(eventCh)
	}()

	return nil
}

func (in *ObjectStatusReporter) Run(stopCh <-chan struct{}, echan <-chan watch.Event, rh cache.ResourceEventHandler) {
	for {
		select {
		case <-stopCh:
			return
		case e, ok := <-echan:
			if !ok {
				klog.Error("event channel closed")
				return
			}

			switch e.Type {
			case watch.Added:
				un, _ := common.ToUnstructured(e.Object)
				rh.OnAdd(un, true)
			case watch.Modified:
				un, _ := common.ToUnstructured(e.Object)
				rh.OnUpdate(nil, un)
			case watch.Deleted:
				un, _ := common.ToUnstructured(e.Object)
				rh.OnDelete(un)
			case watch.Error:
			default:
				klog.V(5).InfoS("unexpected watch event", "event", e)
			}
		}
	}
}

func (in *ObjectStatusReporter) forEachTargetWithGroupKind(gk schema.GroupKind, fn func(GroupKindNamespace)) {
	for gkn := range in.gk2gkn[gk] {
		fn(gkn)
	}
}

func (in *ObjectStatusReporter) forEachTargetWithNamespace(ns string, fn func(GroupKindNamespace)) {
	for gkn := range in.ns2gkn[ns] {
		fn(gkn)
	}
}

// readStatusFromObject is a convenience function to read object status with a
// StatusReader using a ClusterReader to retrieve generated objects.
func (in *ObjectStatusReporter) readStatusFromObject(
	ctx context.Context,
	obj *unstructured.Unstructured,
) (*event.ResourceStatus, error) {
	return in.StatusReader.ReadStatusForObject(ctx, in.ClusterReader, obj)
}

// readStatusFromCluster is a convenience function to read object status with a
// StatusReader using a ClusterReader to retrieve the object and its generated
// objects.
func (in *ObjectStatusReporter) readStatusFromCluster(
	ctx context.Context,
	id object.ObjMetadata,
) (*event.ResourceStatus, error) {
	return in.StatusReader.ReadStatus(ctx, in.ClusterReader, id)
}

// deletedStatus builds a ResourceStatus for a deleted object.
//
// StatusReader.ReadStatusForObject doesn't handle nil objects as input. So
// this builds the status manually.
// TODO: find a way to delegate this back to the status package.
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

// eventHandler builds an event handler to compute object status.
// Returns an event channel on which these stats updates will be reported.
func (in *ObjectStatusReporter) eventHandler(
	ctx context.Context,
	eventCh chan<- event.Event,
) cache.ResourceEventHandler {
	var handler cache.ResourceEventHandlerFuncs

	handler.AddFunc = func(iobj interface{}) {
		// Bail early if the context is cancelled, to avoid unnecessary work.
		if ctx.Err() != nil {
			return
		}

		obj, ok := iobj.(*unstructured.Unstructured)
		if !ok {
			panic(fmt.Sprintf("AddFunc received unexpected object type %T", iobj))
		}
		id := object.UnstructuredToObjMetadata(obj)
		if in.ObjectFilter != nil && in.ObjectFilter.Filter(obj) {
			klog.V(7).Infof("Watch Event Skipped: AddFunc: %s", id)
			return
		}
		klog.V(5).Infof("AddFunc: Computing status for object: %s", id)

		// cancel any scheduled status update for this object
		in.taskManager.Cancel(id)

		rs, err := in.readStatusFromObject(ctx, obj)
		if err != nil {
			// Send error event and stop the reporter!
			in.handleFatalError(eventCh, fmt.Errorf("failed to compute object status: %s: %w", id, err))
			return
		}

		if object.IsNamespace(obj) {
			klog.V(5).Infof("AddFunc: Namespace added: %v", id)
			in.onNamespaceAdd(obj)
		} else if object.IsCRD(obj) {
			klog.V(5).Infof("AddFunc: CRD added: %v", id)
			in.onCRDAdd(obj)
		}

		if isObjectUnschedulable(rs) {
			klog.V(5).Infof("AddFunc: object unschedulable: %v", id)
			// schedule delayed status update
			in.taskManager.Schedule(ctx, id, status.ScheduleWindow,
				in.newStatusCheckTaskFunc(ctx, eventCh, id))
		}

		klog.V(7).Infof("AddFunc: sending update event: %v", rs)
		eventCh <- event.Event{
			Type:     event.ResourceUpdateEvent,
			Resource: rs,
		}
	}

	handler.UpdateFunc = func(_, iobj interface{}) {
		// Bail early if the context is cancelled, to avoid unnecessary work.
		if ctx.Err() != nil {
			return
		}

		obj, ok := iobj.(*unstructured.Unstructured)
		if !ok {
			panic(fmt.Sprintf("UpdateFunc received unexpected object type %T", iobj))
		}
		id := object.UnstructuredToObjMetadata(obj)
		if in.ObjectFilter != nil && in.ObjectFilter.Filter(obj) {
			klog.V(7).Infof("UpdateFunc: Watch Event Skipped: %s", id)
			return
		}
		klog.V(5).Infof("UpdateFunc: Computing status for object: %s", id)

		// cancel any scheduled status update for this object
		in.taskManager.Cancel(id)

		rs, err := in.readStatusFromObject(ctx, obj)
		if err != nil {
			// Send error event and stop the reporter!
			in.handleFatalError(eventCh, fmt.Errorf("failed to compute object status: %s: %w", id, err))
			return
		}

		if object.IsNamespace(obj) {
			klog.V(5).Infof("UpdateFunc: Namespace updated: %v", id)
			in.onNamespaceUpdate(obj)
		} else if object.IsCRD(obj) {
			klog.V(5).Infof("UpdateFunc: CRD updated: %v", id)
			in.onCRDUpdate(obj)
		}

		if isObjectUnschedulable(rs) {
			klog.V(5).Infof("UpdateFunc: object unschedulable: %v", id)
			// schedule delayed status update
			in.taskManager.Schedule(ctx, id, status.ScheduleWindow,
				in.newStatusCheckTaskFunc(ctx, eventCh, id))
		}

		klog.V(7).Infof("UpdateFunc: sending update event: %v", rs)
		eventCh <- event.Event{
			Type:     event.ResourceUpdateEvent,
			Resource: rs,
		}
	}

	handler.DeleteFunc = func(iobj interface{}) {
		// Bail early if the context is cancelled, to avoid unnecessary work.
		if ctx.Err() != nil {
			return
		}

		if tombstone, ok := iobj.(cache.DeletedFinalStateUnknown); ok {
			// Last state unknown. Possibly stale.
			// TODO: Should we propegate this uncertainty to the caller?
			iobj = tombstone.Obj
		}
		obj, ok := iobj.(*unstructured.Unstructured)
		if !ok {
			panic(fmt.Sprintf("DeleteFunc received unexpected object type %T", iobj))
		}
		id := object.UnstructuredToObjMetadata(obj)
		if in.ObjectFilter != nil && in.ObjectFilter.Filter(obj) {
			klog.V(7).Infof("DeleteFunc: Watch Event Skipped: %s", id)
			return
		}
		klog.V(5).Infof("DeleteFunc: Computing status for object: %s", id)

		// cancel any scheduled status update for this object
		in.taskManager.Cancel(id)

		if object.IsNamespace(obj) {
			klog.V(5).Infof("DeleteFunc: Namespace deleted: %v", id)
			in.onNamespaceDelete(obj)
		} else if object.IsCRD(obj) {
			klog.V(5).Infof("DeleteFunc: CRD deleted: %v", id)
			in.onCRDDelete(obj)
		}

		rs := deletedStatus(id)
		klog.V(7).Infof("DeleteFunc: sending update event: %v", rs)
		eventCh <- event.Event{
			Type:     event.ResourceUpdateEvent,
			Resource: rs,
		}
	}

	return handler
}

// onCRDAdd handles creating a new informer to watch the new resource type.
func (in *ObjectStatusReporter) onCRDAdd(obj *unstructured.Unstructured) {
	gk, found := object.GetCRDGroupKind(obj)
	if !found {
		id := object.UnstructuredToObjMetadata(obj)
		klog.Warningf("Invalid CRD added: missing group and/or kind: %v", id)
		// Don't return an error, because this should not inturrupt the task queue.
		// TODO: Allow non-fatal errors to be reported using a specific error type.
		return
	}
	klog.V(3).Infof("CRD added for %s", gk)

	klog.V(3).Info("Resetting RESTMapper")
	// Reset mapper to invalidate cache.
	meta.MaybeResetRESTMapper(in.Mapper)

	in.forEachTargetWithGroupKind(gk, func(gkn GroupKindNamespace) {
		in.startInformer(gkn)
	})
}

// onCRDUpdate handles creating a new informer to watch the updated resource type.
func (in *ObjectStatusReporter) onCRDUpdate(newObj *unstructured.Unstructured) {
	gk, found := object.GetCRDGroupKind(newObj)
	if !found {
		id := object.UnstructuredToObjMetadata(newObj)
		klog.Warningf("Invalid CRD updated: missing group and/or kind: %v", id)
		// Don't return an error, because this should not inturrupt the task queue.
		// TODO: Allow non-fatal errors to be reported using a specific error type.
		return
	}
	klog.V(3).Infof("CRD updated for %s", gk)

	klog.V(3).Info("Resetting RESTMapper")
	// Reset mapper to invalidate cache.
	meta.MaybeResetRESTMapper(in.Mapper)

	in.forEachTargetWithGroupKind(gk, func(gkn GroupKindNamespace) {
		in.startInformer(gkn)
	})
}

// onCRDDelete handles stopping the informer watching the deleted resource type.
func (in *ObjectStatusReporter) onCRDDelete(oldObj *unstructured.Unstructured) {
	gk, found := object.GetCRDGroupKind(oldObj)
	if !found {
		id := object.UnstructuredToObjMetadata(oldObj)
		klog.Warningf("Invalid CRD deleted: missing group and/or kind: %v", id)
		// Don't return an error, because this should not inturrupt the task queue.
		// TODO: Allow non-fatal errors to be reported using a specific error type.
		return
	}
	klog.V(3).Infof("CRD deleted for %s", gk)

	in.forEachTargetWithGroupKind(gk, func(gkn GroupKindNamespace) {
		in.stopInformer(gkn)
	})

	klog.V(3).Info("Resetting RESTMapper")
	// Reset mapper to invalidate cache.
	meta.MaybeResetRESTMapper(in.Mapper)
}

// onNamespaceAdd handles creating new informers to watch this namespace.
func (in *ObjectStatusReporter) onNamespaceAdd(obj *unstructured.Unstructured) {
	if in.RESTScope == meta.RESTScopeRoot {
		// When watching resources across all namespaces,
		// we don't need to start or stop any
		// namespace-specific informers.
		return
	}
	namespace := obj.GetName()
	in.forEachTargetWithNamespace(namespace, func(gkn GroupKindNamespace) {
		in.startInformer(gkn)
	})
}

// onNamespaceUpdate handles creating new informers to watch this namespace.
func (in *ObjectStatusReporter) onNamespaceUpdate(obj *unstructured.Unstructured) {
	if in.RESTScope == meta.RESTScopeRoot {
		// When watching resources across all namespaces,
		// we don't need to start or stop any
		// namespace-specific informers.
		return
	}
	namespace := obj.GetName()
	in.forEachTargetWithNamespace(namespace, func(gkn GroupKindNamespace) {
		in.startInformer(gkn)
	})
}

// onNamespaceDelete handles stopping informers watching this namespace.
func (in *ObjectStatusReporter) onNamespaceDelete(obj *unstructured.Unstructured) {
	if in.RESTScope == meta.RESTScopeRoot {
		// When watching resources across all namespaces,
		// we don't need to start or stop any
		// namespace-specific informers.
		return
	}
	namespace := obj.GetName()
	in.forEachTargetWithNamespace(namespace, func(gkn GroupKindNamespace) {
		in.stopInformer(gkn)
	})
}

// newStatusCheckTaskFunc returns a taskFund that reads the status of an object
// from the cluster and sends it over the event channel.
//
// This method should only be used for generated resource objects, as it's much
// slower at scale than watching the resource for updates.
func (in *ObjectStatusReporter) newStatusCheckTaskFunc(
	ctx context.Context,
	eventCh chan<- event.Event,
	id object.ObjMetadata,
) taskFunc {
	return func() {
		klog.V(5).Infof("Re-reading object status: %v", id)
		// check again
		rs, err := in.readStatusFromCluster(ctx, id)
		if err != nil {
			// Send error event and stop the reporter!
			// TODO: retry N times before terminating
			in.handleFatalError(eventCh, err)
			return
		}
		eventCh <- event.Event{
			Type:     event.ResourceUpdateEvent,
			Resource: rs,
		}
	}
}

func (in *ObjectStatusReporter) handleFatalError(eventCh chan<- event.Event, err error) {
	klog.Warningf("Reporter error: %v", err)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	eventCh <- event.Event{
		Type:  event.ErrorEvent,
		Error: err,
	}
	in.Stop()
}
