package cache

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/object"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/watcher"
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
	key        ResourceKey
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

func SaveResourceSHA(resource *unstructured.Unstructured, shaType SHAType) {
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

func (in *ResourceCache) Register(keys ResourceKeys) {
	keySet := keys.StringSet()
	toDelete := in.toResourceKeysSet().Difference(keySet)
	toAdd := keySet.Difference(in.toResourceKeysSet())

	for key := range toDelete {
		in.stop(key)
	}

	for key := range toAdd {
		resourceKey, err := ParseResourceKey(key)
		if err != nil {
			continue
		}

		in.start(resourceKey)
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

func (in *ResourceCache) start(key ResourceKey) {
	w := watcher.NewDefaultStatusWatcher(in.dynamicClient, in.mapper)
	w.Filters = &watcher.Filters{
		Labels: common.ManagedByAgentLabelSelector(),
		Fields: nil,
	}

	ctx, cancelFunc := context.WithCancel(in.ctx)
	in.resourceToWatcher.Set(key.TypeIdentifier(), &watcherWrapper{
		w:          w,
		key:        key,
		ctx:        ctx,
		cancelFunc: cancelFunc,
	})

	in.startWatch(key.TypeIdentifier())
}

func (in *ResourceCache) startWatch(resourceKey string) {
	wrapper, ok := in.resourceToWatcher.Get(resourceKey)
	if !ok {
		return
	}

	go func() {
		// Should retry? Check if context was cancelled or there was an error?
		ch := wrapper.w.Watch(wrapper.ctx, []object.ObjMetadata{wrapper.key.ObjMetadata()}, watcher.Options{
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
		GetHealthCache().Update(e.Resource)

		if !in.shouldCacheResource(e.Resource) {
			in.deleteCacheEntry(e.Resource)
			return
		}

		SaveResourceSHA(e.Resource.Resource, ServerSHA)
	case event.ErrorEvent:
		in.startWatch(resourceKey)
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
