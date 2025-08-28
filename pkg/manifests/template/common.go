package template

import (
	"errors"
	"fmt"

	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

func setNamespaces(mapper meta.RESTMapper, objs []unstructured.Unstructured,
	defaultNamespace string, enforceNamespace bool) ([]unstructured.Unstructured, error) {
	// find any crds in the set of resources.
	crdObjs := make([]unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if object.IsCRD(&obj) {
			crdObjs = append(crdObjs, obj)
		}
	}

	var unknownGVKs []schema.GroupVersionKind
	for i := range objs {
		// Exclude any inventory objects here since we don't want to change
		// their namespace.
		if inventory.IsInventoryObject(&objs[i]) {
			continue
		}

		gvk := objs[i].GroupVersionKind()
		if namespacedCache.Present(gvk) {
			if namespacedCache.Namespaced(gvk) && objs[i].GetNamespace() == "" {
				objs[i].SetNamespace(defaultNamespace)
			}

			if !namespacedCache.Namespaced(gvk) && objs[i].GetNamespace() != "" {
				objs[i].SetNamespace("")
			}
			continue
		}

		// Look up the scope of the resource so we know if the resource
		// should have a namespace set or not.
		scope, err := object.LookupResourceScope(&objs[i], lo.ToSlicePtr(crdObjs), mapper)
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
			return nil, err
		}

		switch scope {
		case meta.RESTScopeNamespace:
			if objs[i].GetNamespace() == "" {
				objs[i].SetNamespace(defaultNamespace)
			} else {
				ns := objs[i].GetNamespace()
				if enforceNamespace && ns != defaultNamespace {
					return nil, &manifestreader.NamespaceMismatchError{
						Namespace:         ns,
						RequiredNamespace: defaultNamespace,
					}
				}
			}
			namespacedCache.Store(gvk, true)
		case meta.RESTScopeRoot:
			if ns := objs[i].GetNamespace(); ns != "" {
				objs[i].SetNamespace("")
				fmt.Printf("Found cluster scoped resource %s with namespace %s, coerced to un-namespaced\n", objs[i].GetName(), ns)
			}
			namespacedCache.Store(gvk, false)
		default:
			return nil, fmt.Errorf("unknown RESTScope %q", scope.Name())
		}
	}

	if len(unknownGVKs) > 0 {
		err := &manifestreader.UnknownTypesError{
			GroupVersionKinds: unknownGVKs,
		}
		klog.V(log.LogLevelExtended).InfoS("found unknown types", "types", err.GroupVersionKinds, "error", err)
	}

	return objs, nil
}
