package applier

import (
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
)

type FilterFunc func(obj unstructured.Unstructured) bool

// FilterEngine holds a list of filters
type FilterEngine struct {
	filters []FilterFunc
}

// Add adds a new filter
func (fe *FilterEngine) Add(f FilterFunc) {
	fe.filters = append(fe.filters, f)
}

// Match runs all filters and returns true only if all pass
func (fe *FilterEngine) Match(obj unstructured.Unstructured) bool {
	for _, f := range fe.filters {
		if !f(obj) {
			return false
		}
	}
	return true
}

// CacheFilter filters based on whether resources and/or manifests have changed since last applied.
func CacheFilter() FilterFunc {
	return func(obj unstructured.Unstructured) bool {
		serviceID := common.ServiceID(&obj)

		entry, err := streamline.GetGlobalStore().GetComponent(obj)
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "failed to get component from store")
			metrics.Record().ResourceCacheMiss(serviceID)
			return true
		}

		newManifestSHA, err := cache.HashResource(obj)
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "failed to hash resource")
			metrics.Record().ResourceCacheMiss(serviceID)
			return true
		}

		if err := streamline.GetGlobalStore().UpdateComponentSHA(obj, store.TransientManifestSHA); err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "failed to update component SHA")
		}

		if entry.ShouldApply(newManifestSHA) {
			klog.V(log.LogLevelTrace).InfoS("resource requires apply",
				"gvk", obj.GroupVersionKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
			metrics.Record().ResourceCacheMiss(serviceID)
			return true
		}

		klog.V(log.LogLevelTrace).InfoS("resource is cached",
			"gvk", obj.GroupVersionKind(), "name", obj.GetName(), "namespace", obj.GetNamespace())
		metrics.Record().ResourceCacheHit(serviceID)
		return false

	}
}
