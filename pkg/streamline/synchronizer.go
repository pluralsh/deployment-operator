package streamline

import (
	"context"
	"fmt"
	"sync"

	"github.com/samber/lo"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

type Synchronizer interface {
	Start(ctx context.Context) error
	Stop()
	Started() bool
	Resynchronize()
}

type synchronizer struct {
	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc

	client dynamic.Interface

	gvr   schema.GroupVersionResource
	store store.Store
}

func NewSynchronizer(client dynamic.Interface, gvr schema.GroupVersionResource, store store.Store) Synchronizer {
	return &synchronizer{
		gvr:    gvr,
		client: client,
		store:  store,
	}
}

func (in *synchronizer) Start(ctx context.Context) error {
	in.mu.Lock()

	internalCtx, cancel := context.WithCancel(ctx)
	in.cancel = cancel

	list, err := in.client.Resource(in.gvr).Namespace(metav1.NamespaceAll).List(internalCtx, metav1.ListOptions{})
	if err != nil {
		in.mu.Unlock()
		return fmt.Errorf("failed to list resources: %w", err)
	}

	in.handleList(lo.FromPtr(list))

	watchCh, err := in.client.Resource(in.gvr).Namespace(metav1.NamespaceAll).Watch(internalCtx, metav1.ListOptions{
		ResourceVersion: list.GetResourceVersion(),
	})
	if err != nil {
		in.mu.Unlock()
		return fmt.Errorf("failed to start watch: %w", err)
	}

	in.started = true
	in.mu.Unlock()
	klog.V(log.LogLevelVerbose).InfoS("started watching resources", "gvr", in.gvr)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-internalCtx.Done():
			return nil
		case event, ok := <-watchCh.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			in.handleEvent(event)
		}
	}
}

func (in *synchronizer) handleList(list unstructured.UnstructuredList) {
	for _, obj := range list.Items {
		if err := in.store.SaveComponent(obj); err != nil {
			klog.ErrorS(err, "failed to save resource", "gvr", in.gvr, "name", obj.GetName())
		}

		if err := in.store.UpdateComponentSHA(obj, store.ServerSHA); err != nil {
			klog.ErrorS(err, "failed to update component SHA", "gvr", in.gvr)
		}
	}
}

func (in *synchronizer) handleEvent(ev watch.Event) {
	switch ev.Type {
	case watch.Added, watch.Modified:
		obj, err := common.ToUnstructured(ev.Object)
		if err != nil {
			klog.ErrorS(err, "failed to convert to unstructured", "gvr", in.gvr)
			return
		}

		klog.V(log.LogLevelTrace).InfoS("adding/updating resource in the store", "gvr", in.gvr, "name", obj.GetName())
		if err := in.store.SaveComponent(*obj); err != nil {
			klog.ErrorS(err, "failed to save resource", "gvr", in.gvr, "name", obj.GetName())
			return
		}
		if err := in.store.UpdateComponentSHA(lo.FromPtr(obj), store.ServerSHA); err != nil {
			klog.ErrorS(err, "failed to update component SHA", "gvr", in.gvr)
		}
	case watch.Deleted:
		obj, err := common.ToUnstructured(ev.Object)
		if err != nil {
			klog.ErrorS(err, "failed to convert to unstructured", "gvr", in.gvr)
			return
		}

		klog.V(log.LogLevelTrace).InfoS("deleting resource from the store", "gvr", in.gvr, "name", obj.GetName())
		if err = in.store.DeleteComponent(obj.GetUID()); err != nil {
			klog.ErrorS(err, "failed to delete resource", "gvr", in.gvr, "name", obj.GetName())
			return
		}
	}

}

func (in *synchronizer) Stop() {
	in.mu.Lock()
	defer in.mu.Unlock()

	if !in.started {
		return
	}

	// TODO: should we cleanup the store?
	in.cancel()
	in.started = false
}

func (in *synchronizer) Started() bool {
	in.mu.Lock()
	defer in.mu.Unlock()

	return in.started
}

func (in *synchronizer) Resynchronize() {
	panic("implement me") // TODO: Update store with latest data from list. Add/delete resources based on the output.
}
