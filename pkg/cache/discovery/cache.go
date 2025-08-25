package discovery

import (
	"sync"
	"time"

	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

type Cache interface {
	// Add adds the provided GroupVersionKinds to the cache and
	// the corresponding GroupVersionResource as well as the APIVersion.
	Add(...schema.GroupVersionKind)

	// Delete removes the provided GroupVersionKinds from the cache and
	// the corresponding GroupVersionResource as well as the APIVersion.
	Delete(...schema.GroupVersionKind)

	// Refresh force refreshes the cache.
	Refresh() error

	// GroupVersionKind returns the set of GroupVersionKinds in the cache.
	GroupVersionKind() containers.Set[schema.GroupVersionKind]

	// GroupVersionResource returns the set of GroupVersionResources in the cache.
	GroupVersionResource() containers.Set[schema.GroupVersionResource]

	// APIVersions returns the set of APIVersions in the cache.
	APIVersions() containers.Set[string]
}

type cache struct {
	mu sync.RWMutex

	client discovery.DiscoveryInterface
	mapper meta.RESTMapper

	// gvkCache and gvrCache are the main cache.
	// apiVersions is a set of all API versions (group/version) in the cache.
	gvkCache    containers.Set[schema.GroupVersionKind]
	gvrCache    containers.Set[schema.GroupVersionResource]
	apiVersions containers.Set[string]

	// onGroupVersionKindAdded and onGroupVersionKindDeleted are called when a GroupVersionKind
	// is added or deleted from the cache.
	onGroupVersionKindAdded   []GroupVersionKindFunc
	onGroupVersionKindDeleted []GroupVersionKindFunc
}

func (in *cache) Add(gvks ...schema.GroupVersionKind) {
	in.mu.Lock()
	defer in.mu.Unlock()

	for _, entry := range gvks {
		in.add(entry)
	}
}

func (in *cache) Delete(gvks ...schema.GroupVersionKind) {
	in.mu.Lock()
	defer in.mu.Unlock()

	for _, entry := range gvks {
		in.gvkCache.Remove(entry)
		in.apiVersions.Remove(entry.GroupVersion().String())
		klog.V(log.LogLevelDebug).InfoS("deleted gvk from cache", "gvk", entry)

		for _, f := range in.onGroupVersionKindDeleted {
			f(entry)
		}

		gvr, err := in.toGroupVersionResource(entry)
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "unable to map gvk to gvr", "gvk", entry)
			continue
		}

		in.gvrCache.Remove(gvr)
	}
}

func (in *cache) GroupVersionKind() containers.Set[schema.GroupVersionKind] {
	in.mu.RLock()
	defer in.mu.RUnlock()

	return in.gvkCache
}

func (in *cache) GroupVersionResource() containers.Set[schema.GroupVersionResource] {
	in.mu.RLock()
	defer in.mu.RUnlock()

	return in.gvrCache
}

func (in *cache) APIVersions() containers.Set[string] {
	in.mu.RLock()
	defer in.mu.RUnlock()

	return in.apiVersions
}

func (in *cache) Refresh() error {
	in.mu.Lock()
	defer in.mu.Unlock()

	now := time.Now()
	klog.V(log.LogLevelTrace).InfoS("started discovery cache refresh")

	groups, resources, err := in.client.ServerGroupsAndResources()

	// Create temporary cache entries. We will replace the cache
	// entries with the ones from the discovery client
	// once we have successfully retrieved the server resources.
	gvkCache := containers.NewSet[schema.GroupVersionKind]()
	gvrCache := containers.NewSet[schema.GroupVersionResource]()
	apiVersions := containers.NewSet[string]()

	for _, group := range groups {
		for _, version := range group.Versions {
			gvkCache, gvrCache, apiVersions = in.addTo(schema.GroupVersionKind{
				Group:   group.Name,
				Version: lo.Ternary(lo.IsEmpty(version.Version), group.APIVersion, version.Version),
				Kind:    "",
			}, gvkCache, gvrCache, apiVersions)
		}
	}

	for _, resource := range resources {
		gv, err := schema.ParseGroupVersion(resource.GroupVersion)
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "unable to parse group version", "groupVersion", resource.GroupVersion)
			continue
		}

		for _, apiResource := range resource.APIResources {
			if len(apiResource.Verbs) == 0 {
				klog.V(log.LogLevelDebug).InfoS("skipping resource without verbs", "resource", apiResource)
				continue
			}

			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    apiResource.Kind,
			}

			gvkCache, gvrCache, apiVersions = in.addTo(gvk, gvkCache, gvrCache, apiVersions)
		}
	}

	if err == nil {
		deleted := in.gvkCache.Difference(gvkCache)
		for _, entry := range deleted.List() {
			for _, f := range in.onGroupVersionKindDeleted {
				f(entry)
			}
		}

		in.gvkCache = gvkCache
		in.gvrCache = gvrCache
		in.apiVersions = apiVersions
		klog.V(log.LogLevelDebug).InfoS("updated discovery cache")
	}

	klog.V(log.LogLevelTrace).InfoS("finished discovery cache refresh", "duration", time.Since(now))
	// Do not immediately return err since groups and resources
	// might be partially filled in case of error.
	return err
}

func (in *cache) toGroupVersionResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := in.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return mapping.Resource, nil
}

func (in *cache) add(gvk schema.GroupVersionKind) {
	in.gvkCache, in.gvrCache, in.apiVersions = in.addTo(gvk, in.gvkCache, in.gvrCache, in.apiVersions)
}

func (in *cache) addTo(
	gvk schema.GroupVersionKind,
	gvkSet containers.Set[schema.GroupVersionKind],
	gvrSet containers.Set[schema.GroupVersionResource],
	apiVersionSet containers.Set[string],
) (containers.Set[schema.GroupVersionKind], containers.Set[schema.GroupVersionResource], containers.Set[string]) {
	// if kind is empty, we are dealing with a server group and version only, not a resource.
	if len(gvk.Kind) == 0 {
		return gvkSet, gvrSet, in.addAPIVersionTo(gvk.GroupVersion().String(), apiVersionSet)
	}

	return in.addGroupVersionKindTo(gvk, gvkSet),
		in.addGroupVersionResourceTo(gvk, gvrSet),
		in.addAPIVersionTo(gvk.GroupVersion().String(), apiVersionSet)
}

func (in *cache) addAPIVersionTo(apiVersion string, apiVersionSet containers.Set[string]) containers.Set[string] {
	if apiVersionSet.Has(apiVersion) {
		klog.V(log.LogLevelDebug).InfoS("api version already in cache, skipping", "apiVersion", apiVersion)
		return apiVersionSet
	}

	apiVersionSet.Add(apiVersion)
	klog.V(log.LogLevelDebug).InfoS("added api version to cache", "apiVersion", apiVersion)
	return apiVersionSet
}

func (in *cache) addGroupVersionKindTo(gvk schema.GroupVersionKind, gvkSet containers.Set[schema.GroupVersionKind]) containers.Set[schema.GroupVersionKind] {
	if gvkSet.Has(gvk) {
		klog.V(log.LogLevelDebug).InfoS("gvk already in cache, skipping", "gvk", gvk)
		return gvkSet
	}

	gvkSet.Add(gvk)
	klog.V(log.LogLevelDebug).InfoS("added gvk to cache", "gvk", gvk)

	for _, f := range in.onGroupVersionKindAdded {
		f(gvk)
	}

	return gvkSet
}

func (in *cache) addGroupVersionResourceTo(gvk schema.GroupVersionKind, gvrSet containers.Set[schema.GroupVersionResource]) containers.Set[schema.GroupVersionResource] {
	gvr, err := in.toGroupVersionResource(gvk)
	if err != nil {
		klog.V(log.LogLevelExtended).ErrorS(err, "unable to map gvk to gvr", "gvk", gvk)
		return gvrSet
	}

	if gvrSet.Has(gvr) {
		klog.V(log.LogLevelDebug).InfoS("gvr already in cache, skipping", "gvr", gvr)
		return gvrSet
	}

	gvrSet.Add(gvr)
	klog.V(log.LogLevelDebug).InfoS("added gvr to cache", "gvr", gvr)
	return gvrSet
}

type CacheOption func(*cache)

func WithOnGroupVersionKindAdded(f ...GroupVersionKindFunc) CacheOption {
	return func(in *cache) {
		in.onGroupVersionKindAdded = append(in.onGroupVersionKindAdded, f...)
	}
}

func WithOnGroupVersionKindDeleted(f ...GroupVersionKindFunc) CacheOption {
	return func(in *cache) {
		in.onGroupVersionKindDeleted = append(in.onGroupVersionKindDeleted, f...)
	}
}

func NewCache(client discovery.DiscoveryInterface, option ...CacheOption) Cache {
	result := &cache{
		client:                    client,
		mapper:                    restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(client)),
		gvkCache:                  containers.NewSet[schema.GroupVersionKind](),
		gvrCache:                  containers.NewSet[schema.GroupVersionResource](),
		apiVersions:               containers.NewSet[string](),
		onGroupVersionKindAdded:   make([]GroupVersionKindFunc, 0),
		onGroupVersionKindDeleted: make([]GroupVersionKindFunc, 0),
	}

	for _, opt := range option {
		opt(result)
	}

	return result
}

var (
	globalCache      Cache = nil
	globalCacheMutex sync.RWMutex
)

func InitGlobalDiscoveryCache(client discovery.DiscoveryInterface, option ...CacheOption) {
	globalCacheMutex.Lock()
	defer globalCacheMutex.Unlock()

	if globalCache != nil {
		return
	}

	globalCache = NewCache(client, option...)
}

func GlobalCache() Cache {
	globalCacheMutex.RLock()
	defer globalCacheMutex.RUnlock()

	if globalCache == nil {
		klog.Fatalf("global discovery cache is not initialized, call InitGlobalDiscoveryCache first")
	}

	return globalCache
}
