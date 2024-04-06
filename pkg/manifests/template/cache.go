package template

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	namespacedCache = &gvkCache{cache: map[schema.GroupVersionKind]bool{}}
)

type gvkCache struct {
	cache map[schema.GroupVersionKind]bool
}

func (c *gvkCache) Store(gvk schema.GroupVersionKind, namespaced bool) {
	c.cache[gvk] = namespaced
}

func (c *gvkCache) Namespaced(gvk schema.GroupVersionKind) bool {
	val, ok := c.cache[gvk]
	return ok && val
}

func (c *gvkCache) Present(gvk schema.GroupVersionKind) bool {
	_, ok := c.cache[gvk]
	return ok
}
