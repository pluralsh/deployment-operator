package applier

import (
	"context"
	"sync"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

const OwningInventoryKey = "config.k8s.io/owning-inventory"

// TODO: Add skip logic
// TODO: Add dry run support for apply
type Applier struct {
	client dynamic.Interface
	store  store.Store
	mu     sync.Mutex
}

// TODO: use unstructed instead of unstructuredList
func (in *Applier) Apply(ctx context.Context, serviceID string, resources unstructured.UnstructuredList) ([]client.ComponentAttributes, []client.ServiceErrorAttributes, error) {
	resources = in.addServiceAnnotation(resources, serviceID)
	toDelete, err := in.toDelete(serviceID, resources.Items)
	if err != nil {
		return nil, nil, err
	}

	waves := NewWaves(resources)
	waves = append(waves, NewWave(toDelete, DeleteWave))
	componentList := make([]client.ComponentAttributes, 0)
	serviceErrrorList := make([]client.ServiceErrorAttributes, 0)

	for _, wave := range waves {
		processor := NewWaveProcessor(in.client, wave)
		components, errors := processor.Run(ctx)

		componentList = append(componentList, components...)
		serviceErrrorList = append(serviceErrrorList, errors...)
	}

	return componentList, serviceErrrorList, nil
}

func (in *Applier) Destroy(ctx context.Context, serviceID string) ([]client.ComponentAttributes, error) {
	toDelete, err := in.toDelete(serviceID, []unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}

	for _, resource := range toDelete {
		if err = in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Delete(ctx, resource.GetName(), metav1.DeleteOptions{}); !apierrors.IsNotFound(err) {
			return nil, err
		}
	}

	return in.getServiceComponents(serviceID)
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

func (in *Applier) getServiceComponents(serviceID string) ([]client.ComponentAttributes, error) {
	entries, err := in.store.GetByServiceID(serviceID)
	if err != nil {
		return nil, err
	}

	return algorithms.Map(entries, func(entry store.Entry) client.ComponentAttributes {
		return client.ComponentAttributes{
			UID:       lo.ToPtr(entry.UID),
			Group:     entry.Group,
			Kind:      entry.Kind,
			Namespace: entry.Namespace,
			Name:      entry.Name,
			Version:   entry.Version,
			State:     lo.ToPtr(client.ComponentState(entry.Status)),
		}
	}), nil
}

func NewApplier(client dynamic.Interface, store store.Store) *Applier {
	// TODO: figure out default values and options
	return &Applier{client: client, store: store}
}
