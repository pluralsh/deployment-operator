package service

import (
	"context"
	"fmt"
	"reflect"

	"github.com/xeipuuv/gojsonpointer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IgnoreJSONPaths(ctx context.Context, c client.Client, obj unstructured.Unstructured, ignorePaths []string) (unstructured.Unstructured, error) {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(obj.GroupVersionKind())

	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	if err := c.Get(ctx, key, current); err != nil {
		if apierrors.IsNotFound(err) {
			return obj, nil // Object doesn't exist, skip
		}
		return unstructured.Unstructured{}, fmt.Errorf("failed to get live object: %w", err)
	}

	// Compare and overwrite ignored paths
	for _, path := range ignorePaths {
		ptr, err := gojsonpointer.NewJsonPointer(path)
		if err != nil {
			return unstructured.Unstructured{}, fmt.Errorf("invalid JSON pointer %q: %w", path, err)
		}

		liveVal, _, err := ptr.Get(current.Object)
		if err != nil {
			continue // Ignore missing path
		}

		desiredVal, _, err := ptr.Get(obj.Object)
		if err != nil || !reflect.DeepEqual(liveVal, desiredVal) {
			// Set the desired object's field to the live value
			_, err = ptr.Set(obj.Object, liveVal)
			if err != nil {
				return unstructured.Unstructured{}, fmt.Errorf("failed to set path %s: %w", path, err)
			}
		}
	}

	return obj, nil
}

type normalizerKey struct {
	Kind      string
	Name      string
	Namespace string
}

func matchesKey(obj unstructured.Unstructured, key normalizerKey) bool {
	if key.Kind != "" && obj.GetKind() != key.Kind {
		return false
	}
	if key.Name != "" && obj.GetName() != key.Name {
		return false
	}
	if key.Namespace != "" && obj.GetNamespace() != key.Namespace {
		return false
	}
	return true
}
