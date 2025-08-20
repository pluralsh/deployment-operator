package applier

import (
	"context"

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
	toApply, toDelete, err := in.spread(serviceID, resources)
	if err != nil {
		return nil, err
	}

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

func (in *Applier) spread(serviceID string, resources unstructured.UnstructuredList) (toApply []unstructured.Unstructured, toDelete []unstructured.Unstructured, err error) {
	entries, err := in.store.GetByServiceID(serviceID)
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range entries {
		entry.
	}
}

func NewApplier(client dynamic.Interface, store store.Store) *Applier {
	return &Applier{client, store}
}
