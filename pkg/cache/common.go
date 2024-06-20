package cache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ToResourceKey(obj *unstructured.Unstructured) string {
	namespace := obj.GetNamespace()
	gvk := fmt.Sprintf("%s/%s/%s", obj.GroupVersionKind().Group, obj.GroupVersionKind().Version, obj.GroupVersionKind().Kind)
	if len(namespace) == 0 {
		return gvk
	}

	return fmt.Sprintf("%s/%s", obj.GetNamespace(), gvk)
}
