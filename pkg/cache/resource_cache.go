package cache

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/watcher"
)

type ResourceCache struct {
	ctx            context.Context
	dynamicClient  dynamic.Interface
	mapper         meta.RESTMapper
	cache          *Cache[*SHA]
	resourceKeySet containers.Set[string]
	watcher        kwatcher.StatusWatcher
}

var resourceCache *ResourceCache

func Init(ctx context.Context, config *rest.Config) {
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

	w := watcher.NewDefaultStatusWatcher(dynamicClient, mapper, &watcher.Options{
		ObjectFilter:          nil,
		UseCustomObjectFilter: true,
		UseInformerRefCache:   true,
		RESTScopeStrategy:     watcher.RESTScopeRoot,
		Filters: &watcher.Filters{
			Labels: common.ManagedByAgentLabelSelector(),
			Fields: nil,
		},
	})

	resourceCache = &ResourceCache{
		ctx:            ctx,
		dynamicClient:  dynamicClient,
		mapper:         mapper,
		cache:          NewCache[*SHA](ctx, time.Minute*10),
		resourceKeySet: containers.NewSet[string](),
		watcher:        w,
	}
}

func GetResourceCache() *ResourceCache {
	return resourceCache
}

func SaveResourceSHA(resource *unstructured.Unstructured, shaType SHAType) {
	key := object.UnstructuredToObjMetadata(resource).String()
	sha, _ := resourceCache.GetCacheEntry(key)
	if err := sha.SetSHA(*resource, shaType); err == nil {
		resourceCache.SetCacheEntry(key, sha)
	}
}

func (in *ResourceCache) SetCacheEntry(key string, value SHA) {
	in.cache.Set(key, &value)
}

func (in *ResourceCache) GetCacheEntry(key string) (SHA, bool) {
	if sha, exists := in.cache.Get(key); exists && sha != nil {
		return *sha, true
	}

	return SHA{}, false
}

func (in *ResourceCache) Register(inventoryResourceKeys containers.Set[string]) {
	toAdd := inventoryResourceKeys.Difference(in.resourceKeySet)

	if len(toAdd) > 0 {
		in.resourceKeySet = containers.ToSet(append(in.resourceKeySet.List(), inventoryResourceKeys.List()...))
		in.watch()
	}
}

func (in *ResourceCache) watch() {
	objMetadataSet, err := ObjectMetadataSetFromStrings(in.resourceKeySet.List())
	if err != nil {
		log.Logger.Error(err, "unable to get resource keys")
		return
	}

	ch := in.watcher.Watch(in.ctx, objMetadataSet, kwatcher.Options{})

	go func() {
		for {
			select {
			case <-in.ctx.Done():
				if in.ctx.Err() != nil {
					log.Logger.Errorf("status watcher context error %v", in.ctx.Err())
				}
				return
			case e, ok := <-ch:
				if !ok {
					log.Logger.Info("status watcher event channel closed")
					in.watch()
					return
				}
				in.reconcile(e)
			}
		}
	}()
}

func (in *ResourceCache) reconcile(e event.Event) {
	switch e.Type {
	case event.ResourceUpdateEvent:
		if !in.shouldCacheResource(e.Resource) {
			in.deleteCacheEntry(e.Resource)
			return
		}

		SaveResourceSHA(e.Resource.Resource, ServerSHA)
	case event.ErrorEvent:
		// handle
	default:
		// Ignore.
	}
}

func (in *ResourceCache) shouldCacheResource(r *event.ResourceStatus) bool {
	if r == nil {
		return false
	}

	return r.Resource != nil && (r.Status == status.CurrentStatus || r.Status == status.InProgressStatus)
}

func (in *ResourceCache) deleteCacheEntry(r *event.ResourceStatus) {
	if r == nil {
		return
	}

	in.cache.Expire(r.Identifier.String())
}
