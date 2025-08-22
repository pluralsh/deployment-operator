package applier

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
