package cache

import (
	"context"
	"fmt"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/api/meta"
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

type ResourceCache struct {
	ctx               context.Context
	dynamicClient     dynamic.Interface
	mapper            meta.RESTMapper
	resourceToWatcher cmap.ConcurrentMap[string, *watcherWrapper]
	cache             Cache[SHA]
}

var resourceCache *ResourceCache

func (in *ResourceCache) NewCacheEntry(key string) {
	in.cache.Set(key, SHA{})
}

func (in *ResourceCache) GetCacheEntry(key string) (SHA, bool) {
	return in.cache.Get(key)
}

func (in *ResourceCache) Register(resources object.ObjMetadataSet) {
	keySet := ObjMetadataSetToResourceKeys(resources)
	deleteSet := in.toResourceKeysSet().Difference(keySet)
	toAdd := keySet.Difference(in.toResourceKeysSet())

	for key := range deleteSet {
		in.stop(key)
	}

	for key := range toAdd {
		metadata, err := object.ParseObjMetadata(key)
		if err != nil {
			continue
		}

		in.start(metadata)
	}
}

func (in *ResourceCache) toResourceKeysSet() containers.Set[string] {
	return containers.ToSet(in.resourceToWatcher.Keys())
}

func (in *ResourceCache) stop(resourceKey string) {
	w, ok := in.resourceToWatcher.Get(resourceKey)
	if !ok {
		return
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
		in.resourceToWatcher.Remove(resourceKey)
	}
}

func (in *ResourceCache) start(id object.ObjMetadata) {
	w := watcher.NewDefaultStatusWatcher(in.dynamicClient, in.mapper)
	w.Filters = &watcher.Filters{
		Labels: common.ManagedByAgentLabelSelector(),
		Fields: nil,
	}

	key := ObjMetadataToResourceKey(id)
	ctx, cancelFunc := context.WithCancel(context.Background())
	in.resourceToWatcher.Set(key, &watcherWrapper{
		w:          w,
		id:         id,
		ctx:        ctx,
		cancelFunc: cancelFunc,
	})

	in.startWatch(key)
}

func (in *ResourceCache) startWatch(resourceKey string) {
	wrapper, ok := in.resourceToWatcher.Get(resourceKey)
	if !ok {
		return
	}

	go func() {
		// Should retry? Check if context was cancelled or there was an error?
		ch := wrapper.w.Watch(wrapper.ctx, []object.ObjMetadata{wrapper.id}, watcher.Options{
			ObjectFilter:          nil,
			UseCustomObjectFilter: true,
			RESTScopeStrategy:     watcher.RESTScopeRoot,
		})

		for e := range ch {
			in.reconcile(e, resourceKey)
		}
	}()
}

func (in *ResourceCache) reconcile(e event.Event, resourceKey string) {
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

func InitResourceCache(ctx context.Context, mapper meta.RESTMapper, dynamicClient *dynamic.DynamicClient) {
	if resourceCache == nil {
		resourceCache = &ResourceCache{
			ctx:               ctx,
			dynamicClient:     dynamicClient,
			mapper:            mapper,
			resourceToWatcher: cmap.New[*watcherWrapper](),
		}
	}
}

func GetResourceCache() (*ResourceCache, error) {
	if resourceCache == nil {
		return nil, fmt.Errorf("server watcher is not initialized")
	}

	return resourceCache, nil
}
