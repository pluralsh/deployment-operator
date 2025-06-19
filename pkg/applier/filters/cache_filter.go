package filters

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

type CacheFilter struct {
}

// Name returns a filter identifier for logging.
func (c CacheFilter) Name() string {
	return "CacheFilter"
}

func (c CacheFilter) Filter(obj *unstructured.Unstructured) error {
	serviceID := c.serviceID(obj)
	newManifestSHA, err := cache.HashResource(*obj)
	if err != nil {
		// TODO log error
		return nil
	}

	key := cache.ResourceKeyFromUnstructured(obj)
	id := key.ObjectIdentifier()
	entry, exists := cache.GetResourceCache().GetCacheEntry(id)
	if exists && !entry.RequiresApply(newManifestSHA) {
		metrics.Record().ResourceCacheHit(serviceID)
		return fmt.Errorf("skipping cached object %s", id)
	}

	metrics.Record().ResourceCacheMiss(serviceID)
	entry.SetManifestSHA(newManifestSHA)
	cache.GetResourceCache().SetCacheEntry(id, entry)

	return nil
}

func (c CacheFilter) serviceID(obj *unstructured.Unstructured) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		return annotations[inventory.OwningInventoryKey]
	}

	return ""
}
