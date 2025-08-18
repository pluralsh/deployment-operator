package streamline

import (
	"context"
	"fmt"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/polly/containers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"time"

	"sync"

	watchtool "github.com/pluralsh/deployment-operator/pkg/streamline/watch"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func HandleClient(ev watch.Event) {
	obj := ev.Object

	// Try to cast to metav1.Object to access metadata
	if metaObj, ok := obj.(metav1.Object); ok {
		gvk := obj.GetObjectKind().GroupVersionKind()
		klog.Infof("Client1 received event: Type=%s, Kind=%s, Name=%s, Namespace=%s, ResourceVersion=%s",
			ev.Type, gvk.Kind, metaObj.GetName(), metaObj.GetNamespace(), metaObj.GetResourceVersion())
	} else {
		// Fallback if obj does not implement metav1.Object
		klog.Infof("Client received event: Type=%s, Object=%#v", ev.Type, obj)
	}

}

type GlobalWatcher struct {
	mu              sync.Mutex
	discoveryClient *discovery.DiscoveryClient
	dynamicClient   dynamic.Interface
	watchers        map[schema.GroupVersionResource]watchtool.Watcher
	subscribers     map[chan watch.Event]struct{}
}

func (w *GlobalWatcher) StartGlobalWatcherOrDie(ctx context.Context) {
	klog.Info("starting discovery cache")
	err := helpers.BackgroundPollUntilContextCancel(ctx, 1*time.Hour, true, true, func(_ context.Context) (done bool, err error) {
		gvrSet, err := getAllResourceTypes(w.discoveryClient)
		if err != nil {
			klog.Errorf("error getting all resource types: %v", err)
			return false, nil
		}
		for _, gvr := range gvrSet.List() {
			w.EnsureWatch(gvr)
		}

		w.CleanupNotIn(gvrSet)

		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start global watcher in background: %w", err))
	}
}

func NewGlobalWatcher(discoveryClient *discovery.DiscoveryClient, dynamicClient dynamic.Interface) *GlobalWatcher {
	return &GlobalWatcher{
		discoveryClient: discoveryClient,
		dynamicClient:   dynamicClient,
		watchers:        make(map[schema.GroupVersionResource]watchtool.Watcher),
		subscribers:     make(map[chan watch.Event]struct{}),
	}
}

func (gw *GlobalWatcher) Subscribe() <-chan watch.Event {
	ch := make(chan watch.Event)
	gw.mu.Lock()
	gw.subscribers[ch] = struct{}{}
	gw.mu.Unlock()
	return ch
}

func (gw *GlobalWatcher) Unsubscribe(ch <-chan watch.Event) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	for sub := range gw.subscribers {
		if sub == ch {
			delete(gw.subscribers, sub)
			close(sub)
			break
		}
	}
}

func (gw *GlobalWatcher) EnsureWatch(gvr schema.GroupVersionResource) {
	gw.mu.Lock()
	if _, exists := gw.watchers[gvr]; exists {
		gw.mu.Unlock()
		return
	}

	watcher := watchtool.NewWatcher(gw.dynamicClient, gvr)
	gw.watchers[gvr] = watcher
	gw.mu.Unlock()

	watcher.Start(context.Background())

	// Fan-in: forward watcher events to all global subscribers
	go func() {
		subCh := watcher.Subscribe() // get the watcher’s channel

		for ev := range subCh { // read events from this watcher
			gw.mu.Lock()
			for sub := range gw.subscribers {
				select {
				case sub <- ev: // forward to each global subscriber
				default:
					// optional: drop or buffer if subscriber is slow
				}
			}
			gw.mu.Unlock()
		}
	}()
}

func (w *GlobalWatcher) CleanupNotIn(seen containers.Set[schema.GroupVersionResource]) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for gvr, watcher := range w.watchers {
		if !seen.Has(gvr) {
			klog.Info("stopping watcher for (GVK disappeared)", gvr)
			watcher.Stop()
			delete(w.watchers, gvr)
		}
	}
}
