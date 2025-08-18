package watch

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

type Watcher interface {
	Start(ctx context.Context)
	Stop()
	Subscribe() <-chan watch.Event
	Unsubscribe(ch <-chan watch.Event)
}

type GVRWatcher struct {
	gvr    schema.GroupVersionResource
	client dynamic.Interface

	cancel      context.CancelFunc
	internal    chan watch.Event // internal channel from watcher loop
	subscribers map[chan watch.Event]struct{}
	mu          sync.Mutex
}

func NewWatcher(client dynamic.Interface, gvr schema.GroupVersionResource) Watcher {
	return &GVRWatcher{
		gvr:         gvr,
		client:      client,
		internal:    make(chan watch.Event), // unbuffered or small buffer
		subscribers: make(map[chan watch.Event]struct{}),
	}
}

func (w *GVRWatcher) Start(ctx context.Context) {
	var rv string
	backoff := time.Second

	childCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel

	// forwards internal events to all subscribers
	go func() {
		for ev := range w.internal {
			w.mu.Lock()
			for sub := range w.subscribers {
				select {
				case sub <- ev: // blocks if subscriber not reading
				default:
					// optional: drop or buffer
				}
			}
			w.mu.Unlock()
		}
	}()

	// Watch loop
	go func() {
		defer close(w.internal)

		for {
			select {
			case <-childCtx.Done():
				return
			default:
			}

			watchCtx, watchCancel := context.WithCancel(childCtx)
			watchCh, err := w.client.Resource(w.gvr).Namespace(metav1.NamespaceAll).Watch(
				watchCtx,
				metav1.ListOptions{ResourceVersion: rv},
			)
			if err != nil {
				if apierrors.IsResourceExpired(err) {
					rv = ""
				}
				klog.ErrorS(err, "failed to start watch", "gvr", w.gvr)
				watchCancel()
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}

			backoff = time.Second

			for ev := range watchCh.ResultChan() {
				select {
				case w.internal <- ev:
				case <-childCtx.Done():
					watchCancel()
					return
				}
				if obj, ok := ev.Object.(metav1.Object); ok {
					rv = obj.GetResourceVersion()
				}
			}

			watchCancel()
			klog.Info("watch closed, retrying...", "gvr", w.gvr)
			time.Sleep(time.Second)
		}
	}()
}

func (w *GVRWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}

	w.mu.Lock()
	for sub := range w.subscribers {
		close(sub)
	}
	w.subscribers = nil
	w.mu.Unlock()
}

func (w *GVRWatcher) Subscribe() <-chan watch.Event {
	ch := make(chan watch.Event)
	w.mu.Lock()
	w.subscribers[ch] = struct{}{}
	w.mu.Unlock()
	return ch
}

func (w *GVRWatcher) Unsubscribe(ch <-chan watch.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for sub := range w.subscribers {
		if sub == ch {
			delete(w.subscribers, sub)
			close(sub)
			break
		}
	}
}
