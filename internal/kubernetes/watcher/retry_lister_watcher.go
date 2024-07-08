package watcher

import (
	"fmt"

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

	listOptions  metav1.ListOptions
	watchOptions metav1.ListOptions

	resultChan chan apiwatch.Event
}

func (in *RetryListerWatcher) ResultChan() <-chan apiwatch.Event {
	return in.resultChan
}

func (in *RetryListerWatcher) funnel(to chan<- apiwatch.Event, from <-chan apiwatch.Event) {
	for {
		select {
		case <-in.Done():
			return
		case e, ok := <-from:
			if !ok {
				return
			}

			to <- e
		}
	}
}

func (in *RetryListerWatcher) funnelItems(to chan<- apiwatch.Event, items ...apiwatch.Event) {
	for _, item := range items {
		select {
		case <-in.Done():
			klog.V(4).InfoS("funnelItems stopped due to resultChan being closed")
			return
		default:
			to <- item
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
		return nil, err
	}

	// TODO: check if watch supports feeding initial items instead of using list
	if in.isEmptyResourceVersion() {
		klog.V(3).InfoS("starting list and watch as initialResourceVersion is empty")
		err := in.listAndWatch()
		return in, err
	}

	klog.V(3).InfoS("starting watch", "initialResourceVersion", in.initialResourceVersion)
	go in.watch(in.initialResourceVersion)
	return in, nil
}

func (in *RetryListerWatcher) listAndWatch() error {
	list, err := in.listerWatcher.List(in.listOptions)
	if err != nil {
		return fmt.Errorf("error listing resources: %v", err)
	}

	listMetaInterface, err := meta.ListAccessor(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v: %v", list, err)
	}

	resourceVersion := listMetaInterface.GetResourceVersion()
	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v (%v)", list, err)
	}

	go in.watch(resourceVersion, in.toEvents(items...)...)

	return nil
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
	in.funnelItems(in.resultChan, initialItems...)
	in.funnel(in.resultChan, w.ResultChan())
}

func (in *RetryListerWatcher) ensureRequiredArgs() error {
	if in.listerWatcher == nil {
		return fmt.Errorf("listerWatcher must not be nil")
	}

	return nil
}

func NewRetryListerWatcher(options ...RetryListerWatcherOption) (*RetryListerWatcher, error) {
	rw := &RetryListerWatcher{
		resultChan: make(chan apiwatch.Event),
	}

	for _, option := range options {
		option(rw)
	}

	return rw.init()
}
