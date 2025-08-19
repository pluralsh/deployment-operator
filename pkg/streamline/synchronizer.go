package streamline

import (
	"context"
	"fmt"
	"sync"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

type Synchronizer interface {
	Start(ctx context.Context) error
	Stop()
	Resynchronize()
}

type synchronizer struct {
	gvr     schema.GroupVersionResource
	store   store.Store
	client  dynamic.Interface
	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
}

func NewSynchronizer(client dynamic.Interface, gvr schema.GroupVersionResource, store store.Store) Synchronizer {
	return &synchronizer{
		gvr:    gvr,
		client: client,
		store:  store,
	}
}

func (w *synchronizer) Start(ctx context.Context) error {
	list, err := w.client.Resource(w.gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	w.handleList(*list)

	watchCh, err := w.client.Resource(w.gvr).Namespace(metav1.NamespaceAll).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watch: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watchCh.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			w.handleEvent(event)
		}
	}
}

func (w *synchronizer) handleList(list unstructured.UnstructuredList) {
	for _, obj := range list.Items {
		if err := w.store.Save(obj); err != nil {
			klog.ErrorS(err, "failed to save resource", "gvr", w.gvr, "name", obj.GetName())
		}
	}
}

func (w *synchronizer) handleEvent(ev watch.Event) {
	switch ev.Type {
	case watch.Added, watch.Modified:
		obj, err := common.ToUnstructured(ev.Object)
		if err != nil {
			klog.ErrorS(err, "failed to convert to unstructured", "gvr", w.gvr)
			return
		}

		if err := w.store.Save(*obj); err != nil {
			klog.ErrorS(err, "failed to save resource", "gvr", w.gvr, "name", obj.GetName())
		}
	case watch.Deleted:
		obj, err := common.ToUnstructured(ev.Object)
		if err != nil {
			klog.ErrorS(err, "failed to convert to unstructured", "gvr", w.gvr)
			return
		}

		if err = w.store.Delete(obj.GetUID()); err != nil {
			klog.ErrorS(err, "failed to delete resource", "gvr", w.gvr, "name", obj.GetName())
			return
		}
	}

}

func (w *synchronizer) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *synchronizer) Resynchronize() {
	panic("implement me") // TODO: Update store with latest data from list. Add/delete resources based on the output.
}
