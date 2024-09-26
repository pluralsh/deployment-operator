package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	console "github.com/pluralsh/console/go/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

// ResourceCache is responsible for creating a global resource cache of the
// inventory items registered via [ResourceCache.Register] method. In particular, it
// does:
//   - starts unique watches per resource type, watching resource in all namespaces.
//     In order to optimize the number of resources being watched, it uses server-side
//     filtering by label and only watches for resources with specific label. Only
//     registered resource types will be watched.
//   - creates a cache based on watched resources that maps [ResourceKey] to [ResourceCacheEntry].
//     It stores information about latest SHAs calculated during a different reconcile stages as well
//     as simplified resource status. [ServerSHA] is always calculated based on watch events. All other
//     types of SHA ([ManifestSHA], [ApplySHA]) are updated during service reconciliation using [SaveResourceSHA].
//
// TODO: Allow stopping opened watches if any unique resource type gets removed from inventory.
type ResourceCache struct {
	// ctx can be used to stop all tasks running in background.
	ctx context.Context

	// dynamicClient is required to list/watch resources.
	dynamicClient dynamic.Interface

	// mapper helps with extraction of i.e. version based on the group and kind only.
	mapper meta.RESTMapper

	// cache is the main resource cache
	cache *Cache[*ResourceCacheEntry]

	// resourceKeySet stores all registered [ResourceKey] that should be watched.
	// It still contains detailed resource information such as Group/Kind/Name/Namespace,
	// allowing us to uniquely identify resources when creating watches.
	resourceKeySet containers.Set[ResourceKey]

	// watcher is a cli-utils [kwatcher.StatusWatcher] interface.
	// We are using a custom implementation that allows us to better
	// control the lifecycle of opened watches and is using RetryListWatcher
	// instead of informers to minimize the memory footprint.
	watcher kwatcher.StatusWatcher
}

var (
	resourceCache *ResourceCache
	initialized   = false
)

// Init must be executed early in [main] in order to ensure that the
// [ResourceCache] will be initialized properly during the application
// startup.
func Init(ctx context.Context, config *rest.Config, ttl time.Duration) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}

	f := utils.NewFactory(config)
	mapper, err := f.ToRESTMapper()
	if err != nil {
		klog.Error(err, "unable to create rest mapper")
		os.Exit(1)
	}

	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		klog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	w := watcher.NewDynamicStatusWatcher(dynamicClient, discoveryClient, mapper, watcher.Options{
		UseCustomObjectFilter: true,
		ObjectFilter:          nil,
		RESTScopeStrategy:     lo.ToPtr(kwatcher.RESTScopeRoot),
		Filters: &kwatcher.Filters{
			Labels: common.ManagedByAgentLabelSelector(),
			Fields: nil,
		},
		ID: "resource-cache",
	})

	resourceCache = &ResourceCache{
		ctx:            ctx,
		dynamicClient:  dynamicClient,
		mapper:         mapper,
		cache:          NewCache[*ResourceCacheEntry](ctx, ttl),
		resourceKeySet: containers.NewSet[ResourceKey](),
		watcher:        w,
	}

	initialized = true
}

// GetResourceCache returns an instance of [ResourceCache]. It can
// be accessed outside this package only via this getter.
func GetResourceCache() *ResourceCache {
	return resourceCache
}

// GetCacheEntry returns a [ResourceCacheEntry] and an information if it exists.
func (in *ResourceCache) GetCacheEntry(key string) (ResourceCacheEntry, bool) {
	if !initialized {
		klog.V(4).Info("resource cache not initialized")
		return ResourceCacheEntry{}, false
	}

	if sha, exists := in.cache.Get(key); exists && sha != nil {
		return *sha, true
	}

	return ResourceCacheEntry{}, false
}

// SetCacheEntry updates cache key with the provided value of [ResourceCacheEntry].
func (in *ResourceCache) SetCacheEntry(key string, value ResourceCacheEntry) {
	if !initialized {
		klog.V(4).Info("resource cache not initialized")
		return
	}

	in.cache.Set(key, &value)
}

// Register updates watched resources. It uses a set to ensure that only unique resources
// are stored. It only supports registering new resources that are not currently being watched.
// If empty set is provided, it won't do anything.
func (in *ResourceCache) Register(inventoryResourceKeys containers.Set[ResourceKey]) {
	if !initialized {
		klog.V(4).Info("resource cache not initialized")
		return
	}

	toAdd := inventoryResourceKeys.Difference(in.resourceKeySet)

	if len(toAdd) > 0 {
		in.resourceKeySet = containers.ToSet(append(in.resourceKeySet.List(), inventoryResourceKeys.List()...))
		in.watch(toAdd)
	}
}

func (in *ResourceCache) Unregister(inventoryResourceKeys containers.Set[ResourceKey]) {
	if !initialized {
		klog.V(4).Info("resource cache not initialized")
		return
	}

	toRemove := in.resourceKeySet.Difference(inventoryResourceKeys)

	for key := range toRemove {
		in.resourceKeySet.Remove(key)
		// TODO: we should trigger a watch stop too
	}
}

// SaveResourceSHA allows updating specific SHA type based on the provided resource. It will
// calculate the SHA and then update cache.
func SaveResourceSHA(resource *unstructured.Unstructured, shaType SHAType) {
	if !initialized {
		klog.V(4).Info("resource cache not initialized")
		return
	}

	key := object.UnstructuredToObjMetadata(resource).String()
	sha, _ := resourceCache.GetCacheEntry(key)
	if err := sha.SetSHA(*resource, shaType); err == nil {
		resourceCache.SetCacheEntry(key, sha)
	}
}

// GetCacheStatus returns cached status based on the provided key. If no status is found in cache,
// it will make an API call, fetch the latest resource and extract the status.
func (in *ResourceCache) GetCacheStatus(key object.ObjMetadata) (*console.ComponentAttributes, error) {
	if !initialized {
		return nil, fmt.Errorf("resource cache not initialized")
	}

	entry, exists := in.cache.Get(key.String())
	if exists && entry.status != nil {
		return entry.status, nil
	}

	mapping, err := in.mapper.RESTMapping(key.GroupKind)
	if err != nil {
		return nil, err
	}

	gvr := mapping.Resource
	obj, err := in.dynamicClient.Resource(gvr).Namespace(key.Namespace).Get(context.Background(), key.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	s, err := in.toStatusEvent(obj)
	if err != nil {
		return nil, err
	}
	in.saveResourceStatus(obj)
	return common.StatusEventToComponentAttributes(*s, make(map[schema.GroupName]string)), nil
}

func (in *ResourceCache) saveResourceStatus(resource *unstructured.Unstructured) {
	e, err := in.toStatusEvent(resource)
	if err != nil {
		klog.Error(err, "unable to convert resource to status event")
		return
	}

	key := object.UnstructuredToObjMetadata(resource).String()
	cacheEntry, _ := resourceCache.GetCacheEntry(key)
	cacheEntry.SetStatus(*e)
	resourceCache.SetCacheEntry(key, cacheEntry)

}

func (in *ResourceCache) watch(resourceKeySet containers.Set[ResourceKey]) {
	if in.resourceKeySet.Intersect(resourceKeySet).Len() == 0 {
		klog.InfoS("resource keys not found in cache, stopping watch", "resourceKeys", resourceKeySet.List())
		return
	}

	objMetadataSet := ResourceKeys(resourceKeySet.List()).ObjectMetadataSet()
	ch := in.watcher.Watch(in.ctx, objMetadataSet, kwatcher.Options{})

	go func() {
		for {
			select {
			case <-in.ctx.Done():
				if in.ctx.Err() != nil {
					klog.Errorf("status watcher context error %v", in.ctx.Err())
				}
				return
			case e, ok := <-ch:
				if !ok {
					klog.Error("status watcher event channel closed")
					in.watch(resourceKeySet)
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

func (in *ResourceCache) toStatusEvent(resource *unstructured.Unstructured) (*applyevent.StatusEvent, error) {
	sr := statusreaders.NewDefaultStatusReader(in.mapper)
	cr := &clusterreader.DynamicClusterReader{
		DynamicClient: in.dynamicClient,
		Mapper:        in.mapper,
	}
	s, err := sr.ReadStatusForObject(context.Background(), cr, resource)
	if err != nil {
		return nil, err
	}
	return &applyevent.StatusEvent{
		Identifier:       ResourceKeyFromUnstructured(resource).ObjMetadata(),
		PollResourceInfo: s,
		Resource:         resource,
	}, nil
}
