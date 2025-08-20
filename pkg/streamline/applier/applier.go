package applier

import (
	"context"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

const OwningInventoryKey = "config.k8s.io/owning-inventory"

type Applier struct {
	client dynamic.Interface
	store  store.Store
}

func (in *Applier) Run(ctx context.Context, serviceID string, resources unstructured.UnstructuredList) ([]client.ComponentAttributes, error) {
	resources = in.addServiceAnnotation(resources, serviceID)
	toDelete, err := in.toDelete(serviceID, resources.Items)
	if err != nil {
		return nil, err
	}
	
	waves := NewWaves(resources)

	return nil, nil
}

func (in *Applier) addServiceAnnotation(resources unstructured.UnstructuredList, serviceID string) unstructured.UnstructuredList {
	for _, obj := range resources.Items {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		annotations[OwningInventoryKey] = serviceID
		obj.SetAnnotations(annotations)
	}

	return resources
}

func (in *Applier) toDelete(serviceID string, resources []unstructured.Unstructured) (toDelete []unstructured.Unstructured, err error) {
	entries, err := in.store.GetByServiceID(serviceID)
	if err != nil {
		return
	}
	resourceKeys := containers.NewSet[Key]()
	for _, obj := range resources {
		resourceKeys.Add(NewKeyFromUnstructured(obj))
	}

	for _, entry := range entries {
		entryKey := NewKeyFromEntry(entry)
		if !resourceKeys.Has(entryKey) {
			obj := unstructured.Unstructured{}
			obj.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   entry.Group,
				Version: entry.Version,
				Kind:    entry.Kind,
			})
			obj.SetNamespace(entry.Namespace)
			obj.SetName(entry.Name)
			obj.SetUID(types.UID(entry.UID))

			toDelete = append(toDelete, obj)
		}
	}
	return
}

func NewApplier(client dynamic.Interface, store store.Store) *Applier {
	return &Applier{client, store}
}
