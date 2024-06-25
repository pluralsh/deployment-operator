package filters

import (
	"fmt"

	"github.com/pluralsh/deployment-operator/pkg/cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type CacheFilter struct {
}

// Name returns a filter identifier for logging.
func (c CacheFilter) Name() string {
	return "CacheFilter"
}

func (c CacheFilter) Filter(obj *unstructured.Unstructured) error {
	newManifestSHA, err := cache.HashResource(*obj)
	if err != nil {
		// TODO log error
		return nil
	}

	key := cache.ResourceKeyFromUnstructured(obj)
	sha, exists := cache.GetResourceCache().GetCacheEntry(key.ObjectIdentifier())
	if exists && !sha.RequiresApply(newManifestSHA) {
		return fmt.Errorf("skipping cached object %s", key.ObjectIdentifier())
	}

	sha.SetManifestSHA(newManifestSHA)
	cache.GetResourceCache().SetCacheEntry(key.ObjectIdentifier(), sha)

	return nil
}
