package applier

import (
	"context"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/common"

	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

type Applier struct {
	filters   *FilterEngine
	client    dynamic.Interface
	store     store.Store
	waveDelay time.Duration

	// onApply callback is called after each resource is applied
	onApply func(unstructured.Unstructured)
}

func (in *Applier) Apply(ctx context.Context,
	service client.ServiceDeploymentForAgent,
	resources []unstructured.Unstructured,
	opts ...WaveProcessorOption,
) ([]client.ComponentAttributes, []client.ServiceErrorAttributes, error) {
	now := time.Now()

	resources = in.addServiceAnnotation(resources, service.ID)
	toDelete, toApply, err := in.toDelete(service.ID, resources)
	if err != nil {
		return nil, nil, err
	}

	toApply, toSkip := in.filterResources(toApply, lo.FromPtr(service.DryRun))

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
		components, serviceErrors := processor.Run(ctx)

		componentList = append(componentList, components...)
		serviceErrrorList = append(serviceErrrorList, serviceErrors...)

		time.Sleep(in.waveDelay)
	}

	klog.V(log.LogLevelDefault).InfoS(
		"apply result",
		"service", service.Name,
		"id", service.ID,
		"attempted", len(resources),
		"applied", len(componentList)-len(toDelete),
		"deleted", len(toDelete),
		"skipped", len(toSkip),
		"failed", len(serviceErrrorList),
		"dryRun", lo.FromPtr(service.DryRun),
		"duration", time.Since(now),
	)

	for _, resource := range toSkip {
		var compAttr *client.ComponentAttributes
		cacheEntry, err := in.store.GetComponent(resource)

		if err != nil || cacheEntry == nil {
			live, err := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
			if err != nil {
				klog.V(log.LogLevelExtended).ErrorS(err, "failed to get component from cache", "resource", resource)
				continue
			}
			compAttr = common.ToComponentAttributes(live)

			if err := in.store.SaveComponent(*live); err != nil {
				klog.V(log.LogLevelExtended).ErrorS(err, "failed to save component to cache", "resource", resource)
			}
		} else {
			compAttr = lo.ToPtr(cacheEntry.ToComponentAttributes())
		}

		componentList = append(componentList, lo.FromPtr(compAttr))
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
	deleted := 0
	toDelete, _, err := in.toDelete(serviceID, []unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}

	for _, resource := range toDelete {
		live, err := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
		if errors.IsNotFound(err) {
			if err := in.store.DeleteComponent(smcommon.NewStoreKeyFromUnstructured(resource)); err != nil {
				klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component from store", "resource", resource.GetUID())
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		if live.GetAnnotations() != nil && live.GetAnnotations()[smcommon.LifecycleDeleteAnnotation] == smcommon.PreventDeletion {
			if err := in.store.DeleteComponent(smcommon.NewStoreKeyFromUnstructured(lo.FromPtr(live))); err != nil {
				klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component from store", "resource", live.GetUID())
			}

			// Delete service ID annotation so it will not be synced to store.
			annotations := live.GetAnnotations()
			delete(annotations, smcommon.OwningInventoryKey)
			live.SetAnnotations(annotations)
			if _, err := in.client.Resource(helpers.GVRFromGVK(live.GroupVersionKind())).
				Update(ctx, live, metav1.UpdateOptions{}); err != nil {
				return nil, err
			}

			continue
		}

		err = in.client.
			Resource(helpers.GVRFromGVK(live.GroupVersionKind())).
			Namespace(live.GetNamespace()).Delete(ctx, live.GetName(), metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			if err := in.store.DeleteComponent(smcommon.NewStoreKeyFromUnstructured(lo.FromPtr(live))); err != nil {
				klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component from store", "resource", live.GetUID())
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		deleted++
	}

	defer klog.V(log.LogLevelDefault).InfoS("deleted service components", "deleted", deleted, "service", serviceID)
	return in.getServiceComponents(serviceID)
}

func (in *Applier) filterResources(resources []unstructured.Unstructured, dryRun bool) (toApply []unstructured.Unstructured, toSkip []unstructured.Unstructured) {
	for _, resource := range resources {
		// In case of dry run we want to skip the cache filter.
		if in.filters.MatchOmit(resource, lo.Ternary(dryRun, []Filter{FilterCache}, []Filter{})...) {
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

		annotations[smcommon.OwningInventoryKey] = serviceID
		obj.SetAnnotations(annotations)
	}

	return resources
}

func (in *Applier) toDelete(serviceID string, resources []unstructured.Unstructured) (toDelete []unstructured.Unstructured, toApply []unstructured.Unstructured, err error) {
	entries, err := in.store.GetServiceComponents(serviceID)
	if err != nil {
		return
	}

	resourceKeys := containers.NewSet[smcommon.Key]()
	deleteKeys := containers.NewSet[smcommon.Key]()
	resourceKeyToResource := make(map[smcommon.Key]unstructured.Unstructured)
	for _, obj := range resources {
		key := smcommon.NewStoreKeyFromUnstructured(obj).VersionlessKey()
		resourceKeys.Add(key)
		resourceKeyToResource[key] = obj
	}

	for _, entry := range entries {
		entryKey := smcommon.NewStoreKeyFromEntry(entry).VersionlessKey()
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
			deleteKeys.Add(entryKey)
		}
	}

	for _, resource := range resources {
		key := smcommon.NewStoreKeyFromUnstructured(resource).VersionlessKey()
		if deleteKeys.Has(key) {
			continue
		}

		toApply = append(toApply, resource)
	}

	return
}

func (in *Applier) getServiceComponents(serviceID string) ([]client.ComponentAttributes, error) {
	entries, err := in.store.GetServiceComponents(serviceID)
	if err != nil {
		return nil, err
	}

	return algorithms.Map(entries, func(entry smcommon.Entry) client.ComponentAttributes {
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

type Option func(*Applier)

func WithWaveDelay(d time.Duration) Option {
	return func(a *Applier) {
		a.waveDelay = d
	}
}

func WithFilter(name Filter, f FilterFunc) Option {
	return func(a *Applier) {
		a.filters.Add(name, f)
	}
}

func WithOnApply(f func(unstructured.Unstructured)) Option {
	return func(a *Applier) {
		a.onApply = f
	}
}

func NewApplier(client dynamic.Interface, store store.Store, opts ...Option) *Applier {
	result := &Applier{
		client:    client,
		store:     store,
		filters:   NewFilterEngine(),
		waveDelay: 1 * time.Second,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}
