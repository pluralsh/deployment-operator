package watch

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/workqueue"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/polly/containers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type GlobalEvent struct {
	GVR    schema.GroupVersionResource
	Type   watch.EventType
	Object runtime.Object
}

type GlobalWatcher struct {
	mu              sync.Mutex
	config          *rest.Config
	discoveryClient *discovery.DiscoveryClient
	dynamicClient   dynamic.Interface
	watchers        map[schema.GroupVersionResource]context.CancelFunc
	queue           workqueue.TypedRateLimitingInterface[GlobalEvent]
	handleEvent     func(GlobalEvent)
}

func DefaultHandleEvent(ev GlobalEvent) {
	klog.Info("gvr=", ev.GVR, " type=", ev.Type, " kind=", ev.Object.GetObjectKind().GroupVersionKind().Kind)
}

func (w *GlobalWatcher) RunWorkers(ctx context.Context, numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			for {
				obj, shutdown := w.queue.Get()
				if shutdown {
					return
				}
				w.handleEvent(obj)
			}
		}(i)
	}

	// Ensure workers stop on ctx cancellation
	go func() {
		<-ctx.Done()
		w.queue.ShutDown()
	}()
}

func NewGlobalWatcher(discoveryClient *discovery.DiscoveryClient, dynamicClient dynamic.Interface, hadleEvent func(GlobalEvent)) *GlobalWatcher {
	// Create a bucket rate limiter
	typedRateLimiter := workqueue.NewTypedMaxOfRateLimiter(workqueue.NewTypedItemExponentialFailureRateLimiter[GlobalEvent](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[GlobalEvent]{Limiter: rate.NewLimiter(rate.Limit(10), 50)},
	)

	return &GlobalWatcher{
		discoveryClient: discoveryClient,
		dynamicClient:   dynamicClient,
		watchers:        make(map[schema.GroupVersionResource]context.CancelFunc),
		queue:           workqueue.NewTypedRateLimitingQueue(typedRateLimiter),
		handleEvent:     hadleEvent,
	}
}

func (w *GlobalWatcher) StartGlobalWatcherOrDie(ctx context.Context) {
	klog.Info("starting discovery cache")
	w.RunWorkers(ctx, 1)
	err := helpers.BackgroundPollUntilContextCancel(ctx, 1*time.Hour, true, true, func(_ context.Context) (done bool, err error) {
		gvrSet, err := getAllResourceTypes(w.discoveryClient)
		if err != nil {
			klog.Errorf("error getting all resource types: %v", err)
			return false, nil
		}
		for _, gvr := range gvrSet.List() {
			w.EnsureWatch(w.dynamicClient, gvr)
		}

		w.CleanupNotIn(gvrSet)

		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start global watcher in background: %w", err))
	}
}

func (w *GlobalWatcher) EnsureWatch(dynamicClient dynamic.Interface, gvr schema.GroupVersionResource) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.watchers[gvr]; exists {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.watchers[gvr] = cancel

	go func() {
		defer func() {
			w.mu.Lock()
			delete(w.watchers, gvr)
			w.mu.Unlock()
		}()

		var rv string
		backoff := time.Second

		for {
			select {
			case <-ctx.Done():
				klog.Info("stopping watcher for", gvr)
				return
			default:
			}

			watchCtx, watchCancel := context.WithCancel(ctx)
			watchCh, err := dynamicClient.Resource(gvr).Namespace(metav1.NamespaceAll).Watch(watchCtx, metav1.ListOptions{
				ResourceVersion: rv,
			})
			if err != nil {
				// Handle "resource version too old"
				if apierrors.IsResourceExpired(err) {
					klog.Info("resetting RV for", gvr)
					rv = ""
				}

				klog.ErrorS(err, "failed to start watch", "gvr", gvr)
				watchCancel()

				// backoff before retry
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}

			// Reset backoff on success
			backoff = time.Second

			// Consume events
			for ev := range watchCh.ResultChan() {
				w.queue.Add(GlobalEvent{GVR: gvr, Type: ev.Type, Object: ev.Object})
				if obj, ok := ev.Object.(metav1.Object); ok {
					rv = obj.GetResourceVersion()
				}
			}

			// If we exit the loop, the watch closed normally
			watchCancel()
			klog.Info("watch closed, retrying...", "gvr", gvr)

			// Small sleep to avoid hot-looping if server closes immediately
			time.Sleep(time.Second)
		}
	}()
}

func (w *GlobalWatcher) CleanupNotIn(seen containers.Set[schema.GroupVersionResource]) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for gvr, cancel := range w.watchers {
		if !seen.Has(gvr) {
			klog.Info("stopping watcher for (GVK disappeared)", gvr)
			cancel()
			delete(w.watchers, gvr)
		}
	}
}
