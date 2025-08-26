package discovery

import (
	"sync"
	"time"

	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

type CacheUpdateFunc func(schema.GroupVersionKind, schema.GroupVersionResource)

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

	// GroupVersion returns the set of GroupVersions in the cache.
	GroupVersion() containers.Set[schema.GroupVersion]

	// ServerVersion returns the Kubernetes server version.
	ServerVersion() *version.Info

	// OnAdded registers a callback function to be called when a new entry is added to the cache.
	OnAdded(f CacheUpdateFunc)

	// OnDeleted registers a callback function to be called when an entry is deleted from the cache.
	OnDeleted(f CacheUpdateFunc)
}

type cache struct {
	mu sync.RWMutex

	client discovery.DiscoveryInterface
	mapper meta.RESTMapper

	// gvkCache is a set of all GroupVersionKinds in the cache.
	gvkCache containers.Set[schema.GroupVersionKind]

	// gvrCache is a set of all GroupVersionResources in the cache.
	gvrCache containers.Set[schema.GroupVersionResource]

	// gvCache is a set of all API versions (group/version) in the cache.
	gvCache containers.Set[schema.GroupVersion]

	// gvkToGVRMap is maps GroupVersionKind to GroupVersionResource so that we can avoid
	// calling the discovery client for each GroupVersionKind.
	gvkToGVRMap map[schema.GroupVersionKind]schema.GroupVersionResource

	// serverVersion is the Kubernetes server version.
	serverVersion *version.Info

	// onAdded and onDeleted are called when an entry is added to or deleted from the cache.
	onAdded   []CacheUpdateFunc
	onDeleted []CacheUpdateFunc
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
		in.gvCache.Remove(entry.GroupVersion())
		klog.V(log.LogLevelDebug).InfoS("deleted gvk from cache", "gvk", entry)

		gvr, err := in.toGroupVersionResource(entry)
		if in.gvrCache.Has(gvr) {
			for _, f := range in.onDeleted {
				f(entry, gvr)
			}
		}

		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "unable to map gvk to gvr", "gvk", entry)
			continue
		}

		in.gvrCache.Remove(gvr)
	}
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
	gvCache := containers.NewSet[schema.GroupVersion]()

	// reset map as it will be repopulated during refresh
	in.gvkToGVRMap = make(map[schema.GroupVersionKind]schema.GroupVersionResource)

	for _, group := range groups {
		for _, groupVersion := range group.Versions {
			gvkCache, gvrCache, gvCache = in.addTo(schema.GroupVersionKind{
				Group:   group.Name,
				Version: lo.Ternary(lo.IsEmpty(groupVersion.Version), group.APIVersion, groupVersion.Version),
				Kind:    "",
			}, gvkCache, gvrCache, gvCache)
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

			gvkCache, gvrCache, gvCache = in.addTo(gvk, gvkCache, gvrCache, gvCache)
		}
	}

	if err == nil {
		deleted := in.gvkCache.Difference(gvkCache)
		for _, entry := range deleted.List() {
			for _, f := range in.onDeleted {
				f(entry, in.gvkToGVRMap[entry])
			}
		}

		in.gvkCache = gvkCache
		in.gvrCache = gvrCache
		in.gvCache = gvCache
		klog.V(log.LogLevelDebug).InfoS("updated discovery cache")
	}

	in.updateServerVersion()

	klog.V(log.LogLevelTrace).InfoS("finished discovery cache refresh", "duration", time.Since(now))
	// Do not immediately return err since groups and resources
	// might be partially filled in case of error.
	return err
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

func (in *cache) GroupVersion() containers.Set[schema.GroupVersion] {
	in.mu.RLock()
	defer in.mu.RUnlock()

	return in.gvCache
}

func (in *cache) ServerVersion() *version.Info {
	if in.serverVersion == nil {
		in.mu.Lock()
		in.updateServerVersion()
		in.mu.Unlock()
	}

	in.mu.RLock()
	defer in.mu.RUnlock()

	return in.serverVersion
}

func (in *cache) OnAdded(f CacheUpdateFunc) {
	in.mu.Lock()
	defer in.mu.Unlock()

	in.onAdded = append(in.onAdded, f)
}

func (in *cache) OnDeleted(f CacheUpdateFunc) {
	in.mu.Lock()
	defer in.mu.Unlock()

	in.onDeleted = append(in.onDeleted, f)
}

func (in *cache) updateServerVersion() {
	serverVersion, err := in.client.ServerVersion()
	if err != nil {
		klog.V(log.LogLevelExtended).ErrorS(err, "unable to get server version")
		return
	}

	in.serverVersion = serverVersion
	klog.V(log.LogLevelDebug).InfoS("updated server version", "version", in.serverVersion.String())
}

func (in *cache) toGroupVersionResource(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := in.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return mapping.Resource, nil
}

func (in *cache) add(gvk schema.GroupVersionKind) {
	in.gvkCache, in.gvrCache, in.gvCache = in.addTo(gvk, in.gvkCache, in.gvrCache, in.gvCache)
}

func (in *cache) addTo(
	gvk schema.GroupVersionKind,
	gvkSet containers.Set[schema.GroupVersionKind],
	gvrSet containers.Set[schema.GroupVersionResource],
	gvSet containers.Set[schema.GroupVersion],
) (containers.Set[schema.GroupVersionKind], containers.Set[schema.GroupVersionResource], containers.Set[schema.GroupVersion]) {
	// if kind is empty, we are dealing with a server group and version only, not a resource.
	if len(gvk.Kind) == 0 {
		return gvkSet, gvrSet, in.addGroupVersionTo(gvk.GroupVersion(), gvSet)
	}

	return in.addGroupVersionKindTo(gvk, gvkSet),
		in.addGroupVersionResourceTo(gvk, gvrSet),
		in.addGroupVersionTo(gvk.GroupVersion(), gvSet)
}

func (in *cache) addGroupVersionTo(groupVersion schema.GroupVersion, gvCacheSet containers.Set[schema.GroupVersion]) containers.Set[schema.GroupVersion] {
	if gvCacheSet.Has(groupVersion) {
		klog.V(log.LogLevelDebug).InfoS("api version already in cache, skipping", "gv", groupVersion)
		return gvCacheSet
	}

	gvCacheSet.Add(groupVersion)
	klog.V(log.LogLevelDebug).InfoS("added api version to cache", "gv", groupVersion)
	return gvCacheSet
}

func (in *cache) addGroupVersionKindTo(gvk schema.GroupVersionKind, gvkSet containers.Set[schema.GroupVersionKind]) containers.Set[schema.GroupVersionKind] {
	if gvkSet.Has(gvk) {
		klog.V(log.LogLevelDebug).InfoS("gvk already in cache, skipping", "gvk", gvk)
		return gvkSet
	}

	gvkSet.Add(gvk)
	klog.V(log.LogLevelDebug).InfoS("added gvk to cache", "gvk", gvk)

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

	in.gvkToGVRMap[gvk] = gvr
	for _, f := range in.onAdded {
		f(gvk, gvr)
	}

	gvrSet.Add(gvr)
	klog.V(log.LogLevelDebug).InfoS("added gvr to cache", "gvr", gvr)
	return gvrSet
}

type CacheOption func(*cache)

func WithOnAdded(f ...CacheUpdateFunc) CacheOption {
	return func(in *cache) {
		in.onAdded = append(in.onAdded, f...)
	}
}

func WithOnDeleted(f ...CacheUpdateFunc) CacheOption {
	return func(in *cache) {
		in.onDeleted = append(in.onDeleted, f...)
	}
}

func NewCache(client discovery.DiscoveryInterface, option ...CacheOption) Cache {
	result := &cache{
		client:      client,
		mapper:      restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(client)),
		gvkCache:    containers.NewSet[schema.GroupVersionKind](),
		gvrCache:    containers.NewSet[schema.GroupVersionResource](),
		gvCache:     containers.NewSet[schema.GroupVersion](),
		gvkToGVRMap: make(map[schema.GroupVersionKind]schema.GroupVersionResource),
		onAdded:     make([]CacheUpdateFunc, 0),
		onDeleted:   make([]CacheUpdateFunc, 0),
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
