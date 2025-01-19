package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	apiwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/cache"

	"github.com/pluralsh/deployment-operator/internal/metrics"
)

type MapEntry struct {
	ID      string    `json:"ID"`
	Started time.Time `json:"Started"`
}

var startedMap = cmap.New[[]MapEntry]()

func inc(key, id string) {
	entry := MapEntry{ID: id, Started: time.Now()}
	entries := []MapEntry{entry}

	if startedMap.Has(key) {
		entries, _ = startedMap.Get(key)
		entries = append(entries, entry)
	}

	startedMap.Set(key, entries)
}

func dec(key, id string) {
	if !startedMap.Has(key) {
		return
	}

	entries, _ := startedMap.Get(key)
	idx := slices.IndexFunc(entries, func(entry MapEntry) bool {
		return entry.ID == id
	})
	entries = append(entries[:idx], entries[idx+1:]...)
	startedMap.Set(key, entries)
}

func init() {
	go log()
}

func log() {
	_ = wait.PollUntilContextCancel(context.Background(), 4*time.Second, true, func(ctx context.Context) (done bool, err error) {
		mapString, _ := json.MarshalIndent(startedMap, "", "  ")
		klog.V(1).Infof("RetryListerWatcher: %s", mapString)
		return false, nil
	})
}

// RetryListerWatcher is a wrapper around [watch.RetryWatcher]
// that ...
type RetryListerWatcher struct {
	*RetryWatcher

	id                     string
	randId                 string
	initialResourceVersion string
	listerWatcher          cache.ListerWatcher
	ctx                    context.Context

	listOptions metav1.ListOptions
	resultChan  chan apiwatch.Event
}

func (in *RetryListerWatcher) ResultChan() <-chan apiwatch.Event {
	return in.resultChan
}

func (in *RetryListerWatcher) funnel(from <-chan apiwatch.Event) {
	for {
		select {
		case <-in.ctx.Done():
			in.Stop()
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
		case <-in.Done():
			klog.V(4).InfoS("funnelItems stopped due to resultChan being closed")
			return
		case in.resultChan <- lo.FromPtr(item.DeepCopy()):
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
		return nil, err
	}

	// TODO: check if watch supports feeding initial items instead of using list
	//if in.isEmptyResourceVersion() {
	//	klog.V(3).InfoS("starting list and watch as initialResourceVersion is empty")
	//	err := in.listAndWatch()
	//	return in, err
	//}

	klog.V(3).InfoS("starting watch", "initialResourceVersion", in.initialResourceVersion)
	go in.watch(in.initialResourceVersion)
	return in, nil
}

func (in *RetryListerWatcher) listAndWatch() error {
	in.listOptions.ResourceVersion = "0"
	in.listOptions.ResourceVersionMatch = metav1.ResourceVersionMatchNotOlderThan

	list, err := in.listerWatcher.List(in.listOptions)
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	listMetaInterface, err := meta.ListAccessor(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v: %w", list, err)
	}

	resourceVersion := listMetaInterface.GetResourceVersion()
	items, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("unable to understand list result %#v (%w)", list, err)
	}

	go in.watch(resourceVersion, in.toEvents(items...)...)

	return nil
}

// Starts the [watch.RetryWatcher] and funnels all events to our wrapper.
func (in *RetryListerWatcher) watch(resourceVersion string, initialItems ...apiwatch.Event) {
	defer close(in.resultChan)
	metrics.Record().RetryWatcherStart(in.id)
	defer metrics.Record().RetryWatcherStop(in.id)
	inc(in.id, in.randId)
	//klog.V(1).InfoS("RetryListerWatcher(watch): start", "ID", in.ID, "Started", Started)
	defer func() {
		dec(in.id, in.randId)
		//klog.V(1).InfoS("RetryListerWatcher(watch): stop", "ID", in.ID, "Started", Started)
	}()

	w, err := NewRetryWatcher(resourceVersion, in.listerWatcher)
	if err != nil {
		klog.ErrorS(err, "unable to create retry watcher", "resourceVersion", resourceVersion)
		return
	}

	in.RetryWatcher = w
	//in.funnelItems(initialItems...)
	in.funnel(w.ResultChan())
}

func (in *RetryListerWatcher) ensureRequiredArgs() error {
	if in.listerWatcher == nil {
		return fmt.Errorf("listerWatcher must not be nil")
	}

	return nil
}

func NewRetryListerWatcher(ctx context.Context, options ...RetryListerWatcherOption) (*RetryListerWatcher, error) {
	rw := &RetryListerWatcher{
		ctx:        ctx,
		resultChan: make(chan apiwatch.Event),
		randId:     lo.RandomString(8, lo.AlphanumericCharset),
	}

	for _, option := range options {
		option(rw)
	}

	return rw.init()
}
