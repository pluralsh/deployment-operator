package template

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	namespacedCache = &gvkCache{cache: map[schema.GroupVersionKind]bool{}}
	mapMutex        = sync.RWMutex{}
)

type gvkCache struct {
	cache map[schema.GroupVersionKind]bool
}

func (c *gvkCache) Store(gvk schema.GroupVersionKind, namespaced bool) {
	mapMutex.Lock()
	c.cache[gvk] = namespaced
	mapMutex.Unlock()
}

func (c *gvkCache) Namespaced(gvk schema.GroupVersionKind) bool {
	mapMutex.RLock()
	val, ok := c.cache[gvk]
	mapMutex.RUnlock()
	return ok && val
}

func (c *gvkCache) Present(gvk schema.GroupVersionKind) bool {
	mapMutex.RLock()
	_, ok := c.cache[gvk]
	mapMutex.RUnlock()
	return ok
}
