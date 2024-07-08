package cache

import (
	"context"
	"os"
	"time"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/clusterreader"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/statusreaders"

	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	applyevent "sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	kwatcher "sigs.k8s.io/cli-utils/pkg/kstatus/watcher"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/internal/kstatus/watcher"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

type ResourceCache struct {
	ctx            context.Context
	dynamicClient  dynamic.Interface
	mapper         meta.RESTMapper
	cache          *Cache[*ResourceCacheEntry]
	resourceKeySet containers.Set[string]
	watcher        kwatcher.StatusWatcher
}

var resourceCache *ResourceCache

func Init(ctx context.Context, config *rest.Config, ttl time.Duration) {
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

	w := watcher.NewDynamicStatusWatcher(dynamicClient, mapper, watcher.Options{
		UseCustomObjectFilter: true,
		ObjectFilter:          nil,
		UseInformerRefCache:   true,
		RESTScopeStrategy:     lo.ToPtr(kwatcher.RESTScopeRoot),
		Filters: &kwatcher.Filters{
			Labels: common.ManagedByAgentLabelSelector(),
			Fields: nil,
		},
	})

	resourceCache = &ResourceCache{
		ctx:            ctx,
		dynamicClient:  dynamicClient,
		mapper:         mapper,
		cache:          NewCache[*ResourceCacheEntry](ctx, ttl),
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

func (in *ResourceCache) toStatusEvent(resource *unstructured.Unstructured) (*applyevent.StatusEvent, error) {
	sr := statusreaders.NewDefaultStatusReader(in.mapper)
	cr := &clusterreader.DynamicClusterReader{
		DynamicClient: in.dynamicClient,
		Mapper:        in.mapper,
	}
	status, err := sr.ReadStatusForObject(context.Background(), cr, resource)
	if err != nil {
		return nil, err
	}
	return &applyevent.StatusEvent{
		Identifier:       ResourceKeyFromUnstructured(resource).ObjMetadata(),
		PollResourceInfo: status,
		Resource:         resource,
	}, nil
}

func (in *ResourceCache) saveResourceStatus(resource *unstructured.Unstructured) {
	e, err := in.toStatusEvent(resource)
	if err != nil {
		log.Logger.Error(err, "unable to convert resource to status event")
		return
	}
	ca := common.StatusEventToComponentAttributes(*e, make(map[manifests.GroupName]string))
	key := object.UnstructuredToObjMetadata(resource).String()
	cacheEntry, _ := resourceCache.GetCacheEntry(key)
	cacheEntry.SetStatus(ca)
	resourceCache.SetCacheEntry(key, cacheEntry)

}

func (in *ResourceCache) SetCacheEntry(key string, value ResourceCacheEntry) {
	in.cache.Set(key, &value)
}

func (in *ResourceCache) GetCacheEntry(key string) (ResourceCacheEntry, bool) {
	if sha, exists := in.cache.Get(key); exists && sha != nil {
		return *sha, true
	}

	return ResourceCacheEntry{}, false
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
					log.Logger.Error("status watcher event channel closed")
					in.watch()
					return
				}
				in.reconcile(e)
			}
		}
	}()
}

func (in *ResourceCache) reconcile(e event.Event) {
	if e.Type != event.ResourceUpdateEvent {
		return
	}

	if !in.shouldCacheResource(e.Resource) {
		in.deleteCacheEntry(e.Resource)
		return
	}

	SaveResourceSHA(e.Resource.Resource, ServerSHA)
	in.saveResourceStatus(e.Resource.Resource)
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

func (in *ResourceCache) GetCacheStatus(key string) (*console.ComponentAttributes, error) {
	entry, exists := in.cache.Get(key)
	if exists && entry.status != nil {
		return entry.status, nil
	}
	rk, err := ResourceKeyFromString(key)
	if err != nil {
		return nil, err
	}

	mapping, err := in.mapper.RESTMapping(rk.GroupKind)
	if err != nil {
		return nil, err
	}

	gvr := watcher.GvrFromGvk(mapping.GroupVersionKind)
	obj, err := in.dynamicClient.Resource(gvr).Namespace(rk.Namespace).Get(context.Background(), rk.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	s, err := in.toStatusEvent(obj)
	if err != nil {
		return nil, err
	}
	in.saveResourceStatus(obj)
	return common.StatusEventToComponentAttributes(*s, make(map[manifests.GroupName]string)), nil
}
