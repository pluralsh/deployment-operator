package watcher

import (
	"fmt"
	"sync"

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
	*RetryWatcher

	id                     string
	initialResourceVersion string
	listerWatcher          cache.ListerWatcher

	listOptions metav1.ListOptions
	resultChan  chan apiwatch.Event

	stopped  bool
	stopChan chan struct{}
	mutex    sync.Mutex
}

func (in *RetryListerWatcher) ResultChan() <-chan apiwatch.Event {
	return in.resultChan
}

func (in *RetryListerWatcher) Stop() {
	in.mutex.Lock()
	defer in.mutex.Unlock()

	if in.RetryWatcher != nil {
		in.RetryWatcher.Stop()
	}

	in.stopped = true
	close(in.stopChan)
}

func (in *RetryListerWatcher) funnel(from <-chan apiwatch.Event) {
	for {
		select {
		case <-in.stopChan:
			return
		case <-in.Done():
			return
		case e, ok := <-from:
			if !ok {
				return
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

func (in *RetryListerWatcher) init() (*RetryListerWatcher, error) {
	if err := in.ensureRequiredArgs(); err != nil {
		close(in.resultChan)
		return nil, err
	}

	initialItems := make([]apiwatch.Event, 0)
	// TODO: check if watch supports feeding initial items instead of using list
	if in.isEmptyResourceVersion() {
		klog.V(3).InfoS("listing initial resources as initialResourceVersion is empty")

		list, err := in.listerWatcher.List(in.listOptions)
		if err != nil {
			close(in.resultChan)
			return in, fmt.Errorf("error listing resources: %w", err)
		}

		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			close(in.resultChan)
			return in, fmt.Errorf("unable to understand list result %#v: %w", list, err)
		}

		resourceVersion := listMetaInterface.GetResourceVersion()
		items, err := meta.ExtractList(list)
		if err != nil {
			close(in.resultChan)
			return in, fmt.Errorf("unable to understand list result %#v (%w)", list, err)
		}

		in.initialResourceVersion = resourceVersion
		initialItems = in.toEvents(items...)
	}

	klog.V(3).InfoS("starting watch", "resourceVersion", in.initialResourceVersion)
	go in.watch(in.initialResourceVersion, initialItems...)
	return in, nil
}

// Starts the [watch.RetryWatcher] and funnels all events to our wrapper.
func (in *RetryListerWatcher) watch(resourceVersion string, initialItems ...apiwatch.Event) {
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

func (in *RetryListerWatcher) ensureRequiredArgs() error {
	if in.listerWatcher == nil {
		return fmt.Errorf("listerWatcher must not be nil")
	}

	return nil
}

func NewRetryListerWatcher(options ...RetryListerWatcherOption) (*RetryListerWatcher, error) {
	rw := &RetryListerWatcher{
		stopChan:   make(chan struct{}),
		resultChan: make(chan apiwatch.Event),
	}

	for _, option := range options {
		option(rw)
	}

	return rw.init()
}
