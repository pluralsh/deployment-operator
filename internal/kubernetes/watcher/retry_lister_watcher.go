package watcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/pluralsh/polly/algorithms"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/cache"
)

// RetryListerWatcher is a wrapper around [watch.RetryWatcher]
// that ...
type RetryListerWatcher struct {
	sync.Mutex
	*RetryWatcher

	ctx context.Context

	id                     string
	initialResourceVersion string
	listerWatcher          cache.ListerWatcher
	listOptions            metav1.ListOptions

	resultChan chan apiwatch.Event

	stopped  bool
	stopChan chan struct{}
	doneChan chan struct{}
}

func (in *RetryListerWatcher) ResultChan() <-chan apiwatch.Event {
	return in.resultChan
}

func (in *RetryListerWatcher) Stop() {
	if in.RetryWatcher != nil {
		in.RetryWatcher.Stop()
	}

	in.Lock()
	defer in.Unlock()

	if in.stopped {
		return
	}

	in.stopped = true
	close(in.stopChan)
}

func (in *RetryListerWatcher) Done() <-chan struct{} {
	return in.doneChan
}

func (in *RetryListerWatcher) toEvents(objects ...runtime.Object) []apiwatch.Event {
	return algorithms.Map(objects, func(object runtime.Object) apiwatch.Event {
		return apiwatch.Event{
			Type:   apiwatch.Added,
			Object: object,
		}
	})
}

func (in *RetryListerWatcher) isEmptyResourceVersion() bool {
	return len(in.initialResourceVersion) == 0 || in.initialResourceVersion == "0"
}

func (in *RetryListerWatcher) ensureRequiredArgs() error {
	if in.listerWatcher == nil {
		return fmt.Errorf("listerWatcher must not be nil")
	}

	return nil
}

func (in *RetryListerWatcher) funnel(from <-chan apiwatch.Event) {
	span := tracer.StartSpan("funnel", tracer.ResourceName("RetryListerWatcher"), tracer.Tag("id", in.id), tracer.StartTime(time.Now()))
	defer span.Finish(tracer.FinishTime(time.Now()))

	for {
		select {
		case <-in.ctx.Done():
			return
		case <-in.stopChan:
			return
		case e, ok := <-from:
			if !ok || in.stopped {
				return
			}

			select {
			case <-in.ctx.Done():
				return
			default:
			}

			in.resultChan <- e
		}
	}
}

func (in *RetryListerWatcher) funnelItems(items ...apiwatch.Event) {
	for _, item := range items {
		select {
		case <-in.stopChan:
			klog.V(0).InfoS("funnelItems stopped due to stopChan being closed")
			return
		case <-in.Done():
			klog.V(4).InfoS("funnelItems stopped due to resultChan being closed")
			return
		case in.resultChan <- item:
			klog.V(4).InfoS("successfully sent item to resultChan")
		}
	}
}

// Starts the [watch.RetryWatcher] and funnels all events to our wrapper.
func (in *RetryListerWatcher) watch(resourceVersion string, initialItems ...apiwatch.Event) {
	defer close(in.doneChan)
	defer close(in.resultChan)

	w, err := NewRetryWatcher(resourceVersion, in.listerWatcher)
	if err != nil {
		klog.ErrorS(err, "unable to create retry watcher", "resourceVersion", resourceVersion)
		return
	}

	in.RetryWatcher = w
	in.funnelItems(initialItems...)
	in.funnel(w.ResultChan())
}

func (in *RetryListerWatcher) init() (*RetryListerWatcher, error) {
	if err := in.ensureRequiredArgs(); err != nil {
		return nil, err
	}

	initialItems := make([]apiwatch.Event, 0)
	// TODO: check if watch supports feeding initial items instead of using list
	if in.isEmptyResourceVersion() {
		klog.V(3).InfoS("listing initial resources as initialResourceVersion is empty")

		list, err := in.listerWatcher.List(in.listOptions)
		if err != nil {
			return in, fmt.Errorf("error listing resources: %w", err)
		}

		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return in, fmt.Errorf("unable to understand list result %#v: %w", list, err)
		}

		resourceVersion := listMetaInterface.GetResourceVersion()
		items, err := meta.ExtractList(list)
		if err != nil {
			return in, fmt.Errorf("unable to understand list result %#v (%w)", list, err)
		}

		in.initialResourceVersion = resourceVersion
		initialItems = in.toEvents(items...)
	}

	klog.V(3).InfoS("starting watch", "resourceVersion", in.initialResourceVersion)
	go in.watch(in.initialResourceVersion, initialItems...)
	return in, nil
}

func NewRetryListerWatcher(ctx context.Context, options ...RetryListerWatcherOption) (*RetryListerWatcher, error) {
	rw := &RetryListerWatcher{
		resultChan: make(chan apiwatch.Event),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		ctx:        ctx,
	}

	for _, option := range options {
		option(rw)
	}

	return rw.init()
}
