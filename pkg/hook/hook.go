package hook

import (
	resourceutil "github.com/pluralsh/deployment-operator/pkg/sync/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func IsHook(obj *unstructured.Unstructured) bool {
	value, ok := obj.GetAnnotations()[AnnotationKeyHook]
	return ok && value != "crd-install"
}

func Types(obj *unstructured.Unstructured) []Type {
	var types []Type
	for _, text := range resourceutil.GetAnnotationCSVs(obj, AnnotationKeyHook) {
		t, ok := NewHookType(text)
		if ok {
			types = append(types, t)
		}
	}

	return types
}

func SplitHooks(target []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured) {
	targetObjs := make([]*unstructured.Unstructured, 0)
	hooks := make([]*unstructured.Unstructured, 0)
	for _, obj := range target {
		if obj == nil {
			continue
		}
		if IsHook(obj) {
			hooks = append(hooks, obj)
		} else {
			targetObjs = append(targetObjs, obj)
		}
	}
	return targetObjs, hooks
}
