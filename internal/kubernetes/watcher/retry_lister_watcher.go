package watcher

import (
	"context"
	"errors"
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

func (in *RetryListerWatcher) isEmptyResourceVersion() bool {
	return len(in.initialResourceVersion) == 0 || in.initialResourceVersion == "0"
}

func (in *RetryListerWatcher) ensureRequiredArgs() error {
	if in.listerWatcher == nil {
		return errors.New("listerWatcher must not be nil")
	}

	return nil
}

func (in *RetryListerWatcher) funnel(from <-chan apiwatch.Event) {
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
			case <-in.stopChan:
				return
			case in.resultChan <- e:
				// Successfully sent
			}
		}
	}
}

func (in *RetryListerWatcher) funnelItems(items []apiwatch.Event) {
	for _, item := range items {
		select {
		case <-in.ctx.Done():
			klog.V(4).InfoS("funnelItems stopped due to context being closed")
			return
		case <-in.stopChan:
			klog.V(4).InfoS("funnelItems stopped due to stopChan being closed")
			return
		case in.resultChan <- item:
			klog.V(4).InfoS("successfully sent item to resultChan")
		}
	}
}

func (in *RetryListerWatcher) initialItemsList() ([]apiwatch.Event, error) {
	if !in.isEmptyResourceVersion() {
		return []apiwatch.Event{}, nil
	}

	klog.V(3).InfoS("listing initial resources as initialResourceVersion is empty")

	list, err := in.listerWatcher.List(in.listOptions)
	if err != nil {
		return nil, fmt.Errorf("error listing resources: %w", err)
	}

	listMetaInterface, err := meta.ListAccessor(list)
	if err != nil {
		return nil, fmt.Errorf("unable to understand list result %#v: %w", list, err)
	}

	resourceVersion := listMetaInterface.GetResourceVersion()
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, fmt.Errorf("unable to understand list result %#v (%w)", list, err)
	}

	in.initialResourceVersion = resourceVersion
	return algorithms.Map(items, func(object runtime.Object) apiwatch.Event {
		return apiwatch.Event{
			Type:   apiwatch.Added,
			Object: object,
		}
	}), nil
}

// Starts the [watch.RetryWatcher] and funnels all events to our wrapper.
func (in *RetryListerWatcher) watch() {
	defer close(in.doneChan)
	defer close(in.resultChan)

	initialItems, err := in.initialItemsList()
	if err != nil {
		// this is constantly thrown when context is canceled
		klog.V(3).ErrorS(err, "unable to list initial items", "resourceVersion", in.initialResourceVersion)
		return
	}

	w, err := NewRetryWatcher(in.initialResourceVersion, in.listerWatcher)
	if err != nil {
		klog.ErrorS(err, "unable to create retry watcher", "resourceVersion", in.initialResourceVersion)
		return
	}

	in.RetryWatcher = w
	in.funnelItems(initialItems)
	in.funnel(w.ResultChan())
}

func (in *RetryListerWatcher) init() (*RetryListerWatcher, error) {
	if err := in.ensureRequiredArgs(); err != nil {
		return nil, err
	}

	klog.V(3).InfoS("starting watch", "resourceVersion", in.initialResourceVersion)
	go in.watch()
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
