package applier

import (
	"context"
	"sync"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

type Applier struct {
	filters FilterEngine
	client  dynamic.Interface
	store   store.Store
	mu      sync.Mutex
}

func (in *Applier) Apply(ctx context.Context, service client.ServiceDeploymentForAgent, resources []unstructured.Unstructured, opts ...Option) ([]client.ComponentAttributes, []client.ServiceErrorAttributes, error) {
	now := time.Now()

	resources = in.addServiceAnnotation(resources, service.ID)
	toDelete, err := in.toDelete(service.ID, resources)
	if err != nil {
		return nil, nil, err
	}

	toApply, toSkip := in.filterResources(resources)

	waves := NewWaves(toApply)
	waves = append(waves, NewWave(toDelete, DeleteWave))

	// Filter out empty waves
	waves = algorithms.Filter(waves, func(w Wave) bool {
		return w.Len() > 0
	})

	componentList := make([]client.ComponentAttributes, 0)
	serviceErrrorList := make([]client.ServiceErrorAttributes, 0)
	for _, wave := range waves {
		processor := NewWaveProcessor(in.client, wave, opts...)
		components, errors := processor.Run(ctx)

		componentList = append(componentList, components...)
		serviceErrrorList = append(serviceErrrorList, errors...)
		// TODO: wait between waves?
	}

	klog.V(log.LogLevelDefault).InfoS(
		"apply result",
		"service", service.Name,
		"id", service.ID,
		"attempted", len(resources),
		"applied", len(componentList),
		"skipped", len(toSkip),
		"failed", len(serviceErrrorList),
		"duration", time.Since(now),
	)

	for _, resource := range toSkip {
		cacheEntry, err := in.store.GetComponent(resource)
		// TODO: in case of error we should probably read it straight from the api server
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "failed to get component from cache", "resource", resource)
			continue
		}

		componentList = append(componentList, cacheEntry.ToComponentAttributes())
	}

	for idx, component := range componentList {
		children, err := in.store.GetComponentChildren(lo.FromPtr(component.UID))
		if err != nil {
			klog.V(log.LogLevelExtended).ErrorS(err, "failed to get children for component", "component", component.Name)
			continue
		}

		componentList[idx].Children = lo.ToSlicePtr(children)
	}

	return componentList, serviceErrrorList, nil
}

func (in *Applier) Destroy(ctx context.Context, serviceID string) ([]client.ComponentAttributes, error) {
	toDelete, err := in.toDelete(serviceID, []unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}

	for _, resource := range toDelete {
		if err = in.client.
			Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).
			Namespace(resource.GetNamespace()).Delete(ctx, resource.GetName(), metav1.DeleteOptions{}); k8sclient.IgnoreNotFound(err) != nil {
			return nil, err
		}
	}

	return in.getServiceComponents(serviceID)
}

func (in *Applier) filterResources(resources []unstructured.Unstructured) (toApply []unstructured.Unstructured, toSkip []unstructured.Unstructured) {
	for _, resource := range resources {
		if in.filters.Match(resource) {
			toApply = append(toApply, resource)
		} else {
			toSkip = append(toSkip, resource)
		}
	}

	return
}

func (in *Applier) addServiceAnnotation(resources []unstructured.Unstructured, serviceID string) []unstructured.Unstructured {
	for _, obj := range resources {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		annotations[common.OwningInventoryKey] = serviceID
		obj.SetAnnotations(annotations)
	}

	return resources
}

func (in *Applier) toDelete(serviceID string, resources []unstructured.Unstructured) (toDelete []unstructured.Unstructured, err error) {
	entries, err := in.store.GetServiceComponents(serviceID)
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
	entries, err := in.store.GetServiceComponents(serviceID)
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

func NewApplier(client dynamic.Interface, store store.Store, filters ...FilterFunc) *Applier {
	// TODO: figure out default values and options
	return &Applier{client: client, store: store, filters: FilterEngine{filters: filters}}
}
