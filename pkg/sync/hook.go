package sync

import (
	"sort"

	"github.com/pluralsh/deployment-operator/pkg/hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetHooks(obj []*unstructured.Unstructured) []hook.Hook {
	hooks := make([]hook.Hook, 0)
	for _, h := range obj {
		hooks = append(hooks, hook.Hook{
			Weight: hook.Weight(h),
			Types:  hook.Types(h),
			Kind:   h.GetObjectKind(),
			Object: h,
		})
	}
	sort.Slice(hooks, func(i, j int) bool {
		kindI := hooks[i].Kind.GroupVersionKind()
		kindJ := hooks[j].Kind.GroupVersionKind()
		return kindI.Kind < kindJ.Kind
	})
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Weight < hooks[j].Weight
	})

	return hooks
}
