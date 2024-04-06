package template

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func setNamespaces(mapper meta.RESTMapper, objs []*unstructured.Unstructured,
	defaultNamespace string, enforceNamespace bool) error {
	var crdObjs []*unstructured.Unstructured

	// find any crds in the set of resources.
	for _, obj := range objs {
		if object.IsCRD(obj) {
			crdObjs = append(crdObjs, obj)
		}
	}

	var unknownGVKs []schema.GroupVersionKind
	for _, obj := range objs {
		// Exclude any inventory objects here since we don't want to change
		// their namespace.
		if inventory.IsInventoryObject(obj) {
			continue
		}

		gvk := obj.GroupVersionKind()
		if namespacedCache.Present(gvk) {
			if namespacedCache.Namespaced(gvk) && obj.GetNamespace() == "" {
				obj.SetNamespace(defaultNamespace)
			}

			if !namespacedCache.Namespaced(gvk) && obj.GetNamespace() != "" {
				obj.SetNamespace("")
			}
			continue
		}

		// Look up the scope of the resource so we know if the resource
		// should have a namespace set or not.
		scope, err := object.LookupResourceScope(obj, crdObjs, mapper)
		if err != nil {
			var unknownTypeError *object.UnknownTypeError
			if errors.As(err, &unknownTypeError) {
				// If no scope was found, just add the resource type to the list
				// of unknown types.
				unknownGVKs = append(unknownGVKs, unknownTypeError.GroupVersionKind)
				continue
			}
			// If something went wrong when looking up the scope, just
			// give up.
			return err
		}

		switch scope {
		case meta.RESTScopeNamespace:
			if obj.GetNamespace() == "" {
				obj.SetNamespace(defaultNamespace)
			} else {
				ns := obj.GetNamespace()
				if enforceNamespace && ns != defaultNamespace {
					return &manifestreader.NamespaceMismatchError{
						Namespace:         ns,
						RequiredNamespace: defaultNamespace,
					}
				}
			}
			namespacedCache.Store(gvk, true)
		case meta.RESTScopeRoot:
			if ns := obj.GetNamespace(); ns != "" {
				obj.SetNamespace("")
				fmt.Printf("Found cluster scoped resource %s with namespace %s, coerced to un-namespaced\n", obj.GetName(), ns)
			}
			namespacedCache.Store(gvk, false)
		default:
			return fmt.Errorf("unknown RESTScope %q", scope.Name())
		}
	}
	if len(unknownGVKs) > 0 {
		err := &manifestreader.UnknownTypesError{
			GroupVersionKinds: unknownGVKs,
		}
		fmt.Printf("Found unknown types %s, ignoring for now", err)
	}
	return nil
}
