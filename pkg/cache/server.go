package cache

import (
	"context"
	"fmt"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/watcher"
)

type watcherWrapper struct {
	w          watcher.StatusWatcher
	id         object.ObjMetadata
	ctx        context.Context
	cancelFunc context.CancelFunc
}

var defaultServerCache = NewServerCache()

type ServerCache struct {
	dynamicClient     dynamic.Interface
	mapper            meta.RESTMapper
	resourceToWatcher cmap.ConcurrentMap[string, *watcherWrapper]
	expiry            time.Duration
}

func (in *ServerCache) Register(resourceMap map[string]*unstructured.Unstructured) {
	keySet := containers.ToSet(lo.Keys(resourceMap))
	deleteSet := in.toResourceKeysSet().Difference(keySet)
	toAdd := keySet.Difference(in.toResourceKeysSet())

	for key := range deleteSet {
		in.stop(key)
	}

	for key := range toAdd {
		in.start(resourceMap[key])
	}
}

func (in *ServerCache) toResourceKeysSet() containers.Set[string] {
	return containers.ToSet(in.resourceToWatcher.Keys())
}

func (in *ServerCache) stop(resourceKey string) {
	w, ok := in.resourceToWatcher.Get(resourceKey)
	if !ok {
		return
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
		in.resourceToWatcher.Remove(resourceKey)
	}
}

func (in *ServerCache) start(obj *unstructured.Unstructured) {
	resourceKey := ToResourceKey(obj)
	w := watcher.NewDefaultStatusWatcher(in.dynamicClient, in.mapper)
	w.Filters = &watcher.Filters{
		Labels: common.ManagedByAgentLabelSelector(),
		Fields: nil,
	}

	id := object.ObjMetadata{
		GroupKind: schema.GroupKind{
			Group: obj.GroupVersionKind().Group,
			Kind:  obj.GroupVersionKind().Kind,
		},
		Namespace: obj.GetNamespace(),
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	in.resourceToWatcher.Set(resourceKey, &watcherWrapper{
		w:          w,
		id:         id,
		ctx:        ctx,
		cancelFunc: cancelFunc,
	})

	in.startWatch(resourceKey)
}

func (in *ServerCache) startWatch(resourceKey string) {
	wrapper, ok := in.resourceToWatcher.Get(resourceKey)
	if !ok {
		return
	}

	go func() {
		// Should retry? Check if context was cancelled or there was an error?
		ch := wrapper.w.Watch(wrapper.ctx, []object.ObjMetadata{wrapper.id}, watcher.Options{
			ObjectFilter:           nil,
			UseDefaultObjectFilter: false,
		})

		for e := range ch {
			in.reconcile(e, resourceKey)
		}
	}()
}

func (in *ServerCache) reconcile(e event.Event, resourceKey string) {
	switch e.Type {
	case event.ResourceUpdateEvent:
		// update status and fill out the cache
		fmt.Printf("%+v\n", e.Resource)
	case event.SyncEvent:
	case event.ErrorEvent:
		in.startWatch(resourceKey)
		// retry watch based on resourceKey
	}
}

func NewServerCache() *ServerCache {
	return &ServerCache{
		resourceToWatcher: cmap.New[*watcherWrapper](),
	}
}

func DefaultServerCache() *ServerCache {
	return defaultServerCache
}
