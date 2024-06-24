package cache

import (
	"context"
	"fmt"
	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/watcher"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/object"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ResourceCache struct {
	ctx               context.Context
	dynamicClient     dynamic.Interface
	mapper            meta.RESTMapper
	resourceToWatcher cmap.ConcurrentMap[string, *watcherWrapper]
	cache             *Cache[SHA]
}

type watcherWrapper struct {
	w          watcher.StatusWatcher
	id         object.ObjMetadata
	ctx        context.Context
	cancelFunc context.CancelFunc
}

var resourceCache *ResourceCache

func init() {
	ctx := context.Background()
	config := ctrl.GetConfigOrDie()

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Logger.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}

	f := utils.NewFactory(config)
	mapper, err := f.ToRESTMapper()
	if err != nil {
		log.Logger.Error(err, "unable to create rest mapper")
		os.Exit(1)
	}

	resourceCache = &ResourceCache{
		ctx:               ctx,
		dynamicClient:     dynamicClient,
		mapper:            mapper,
		resourceToWatcher: cmap.New[*watcherWrapper](),
		cache:             NewCache[SHA](time.Minute*10, time.Second*30),
	}
}

func GetResourceCache() *ResourceCache {
	return resourceCache
}

func SaveResourceCache(resource *unstructured.Unstructured, shaType SHAType) {
	key := object.UnstructuredToObjMetadata(resource).String()
	sha, _ := resourceCache.GetCacheEntry(key)
	if err := sha.SetSHA(*resource, shaType); err == nil {
		resourceCache.SetCacheEntry(key, sha)
	}
}

func (in *ResourceCache) SetCacheEntry(key string, value SHA) {
	in.cache.Set(key, value)
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
	ctx, cancelFunc := context.WithCancel(in.ctx)
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
		SaveResourceCache(e.Resource.Resource, ServerSHA)
		//if health := getResourceHealth(e.Resource.Resource); health != nil {
		//			sha.health = lo.ToPtr(health.String())
		//		}
	case event.SyncEvent:
	case event.ErrorEvent:
		in.startWatch(resourceKey)
		// retry watch based on resourceKey
	}
}

// getResourceHealth returns the health of a k8s resource
func getResourceHealth(obj *unstructured.Unstructured) *console.ComponentState {
	if obj.GetDeletionTimestamp() != nil {
		return lo.ToPtr(console.ComponentStatePending)
	}

	if healthCheck := common.GetHealthCheckFuncByGroupVersionKind(obj.GroupVersionKind()); healthCheck != nil {
		health, err := healthCheck(obj)
		if err != nil {
			return nil
		}
		if health.Status == common.HealthStatusDegraded {
			return lo.ToPtr(console.ComponentStateFailed)
		}

		if health.Status == common.HealthStatusHealthy {
			return lo.ToPtr(console.ComponentStateRunning)
		}

		if health.Status == common.HealthStatusPaused {
			return lo.ToPtr(console.ComponentStatePaused)
		}
	}
	return lo.ToPtr(console.ComponentStatePending)
}