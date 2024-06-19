package cache

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var APIVersions cmap.ConcurrentMap[string, schema.GroupVersionResource] = cmap.New[schema.GroupVersionResource]()
