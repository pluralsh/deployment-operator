package applier

import (
	"github.com/pluralsh/deployment-operator/pkg/cache"
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
			return true
		}
	}
	return false
}

func SkipFilter() FilterFunc {
	return func(obj unstructured.Unstructured) bool {
		entry, err := streamline.GetGlobalStore().GetComponent(obj)
		if err != nil {
			return true
		}

		newManifestSHA, _ := cache.HashResource(obj)

		if err := streamline.GetGlobalStore().UpdateComponentSHA(obj, store.TransientManifestSHA); err != nil {
			klog.Errorf("Failed to update component SHA: %v", err)
		}

		result := entry.ServerSHA == "" ||
			entry.ApplySHA == "" ||
			entry.ManifestSHA == "" ||
			(entry.ServerSHA != entry.ApplySHA) ||
			(newManifestSHA != entry.ManifestSHA)

		return result
	}
}
