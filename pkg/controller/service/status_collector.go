package service

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

type serviceComponentsStatusCollector struct {
	latestStatus map[object.ObjMetadata]event.StatusEvent
	reconciler   *ServiceReconciler
	applyStatus  map[object.ObjMetadata]event.ApplyEvent
	DryRun       bool
}

func newServiceComponentsStatusCollector(reconciler *ServiceReconciler, svc *console.ServiceDeploymentForAgent) *serviceComponentsStatusCollector {
	if svc.DryRun == nil {
		svc.DryRun = lo.ToPtr(false)
	}
	return &serviceComponentsStatusCollector{
		latestStatus: make(map[object.ObjMetadata]event.StatusEvent),
		applyStatus:  make(map[object.ObjMetadata]event.ApplyEvent),
		reconciler:   reconciler,
		DryRun:       *svc.DryRun,
	}
}

func (sc *serviceComponentsStatusCollector) updateStatus(id object.ObjMetadata, se event.StatusEvent) {
	sc.latestStatus[id] = se
}

func (sc *serviceComponentsStatusCollector) updateApplyStatus(id object.ObjMetadata, se event.ApplyEvent) {
	sc.applyStatus[id] = se
}

func (sc *serviceComponentsStatusCollector) refetch(resource *unstructured.Unstructured) *unstructured.Unstructured {
	if sc.reconciler.clientset == nil || resource == nil {
		return nil
	}

	response := new(unstructured.Unstructured)
	err := sc.reconciler.clientset.RESTClient().Get().AbsPath(toAPIPath(resource)).Do(context.Background()).Into(response)
	if err != nil {
		return nil
	}

	return response
}

func (sc *serviceComponentsStatusCollector) fromApplyResult(e event.ApplyEvent, vcache map[schema.GroupName]string) *console.ComponentAttributes {
	if e.Resource == nil {
		return nil
	}
	gvk := e.Resource.GroupVersionKind()
	gname := schema.GroupName{
		Group: gvk.Group,
		Kind:  gvk.Kind,
		Name:  e.Resource.GetName(),
	}

	version := gvk.Version
	if v, ok := vcache[gname]; ok {
		version = v
	}

	desired := asJSON(e.Resource)
	live := "# n/a"
	liveResource := sc.refetch(e.Resource)
	if liveResource != nil {
		live = asJSON(liveResource)
	}

	return &console.ComponentAttributes{
		UID:       lo.ToPtr(string(e.Resource.GetUID())),
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    live == desired,
		State:     common.ToStatus(e.Resource),
		Content: &console.ComponentContentAttributes{
			Desired: &desired,
			Live:    &live,
		},
	}
}

// func (sc *serviceComponentsStatusCollector) componentsAttributes(vcache map[schema.GroupName]string) []*console.ComponentAttributes {
//	components := make([]*console.ComponentAttributes, 0, len(sc.latestStatus))
//
//	if sc.DryRun {
//		for _, v := range sc.applyStatus {
//			if consoleAttr := sc.fromApplyResult(v, vcache); consoleAttr != nil {
//				components = append(components, consoleAttr)
//			}
//		}
//		return components
//	}
//
//	for _, v := range sc.latestStatus {
//		if attrs := common.StatusEventToComponentAttributes(v, vcache); attrs != nil {
//			components = append(components, attrs)
//		}
//	}
//
//	applyKeys := maps.Keys(sc.applyStatus)
//	statusKeys := maps.Keys(sc.latestStatus)
//	diff := containers.ToSet(applyKeys).Difference(containers.ToSet(statusKeys))
//	for key := range diff {
//		if err := cache.GetResourceCache().SyncCacheStatus(key); err != nil {
//			klog.ErrorS(err, "failed to sync cache status")
//		}
//
//		e, err := cache.GetResourceCache().GetCacheStatus(key)
//		if err != nil {
//			e = &console.ComponentAttributes{
//				State:     lo.ToPtr(console.ComponentStatePending),
//				Synced:    false,
//				Group:     key.GroupKind.Group,
//				Kind:      key.GroupKind.Kind,
//				Namespace: key.Namespace,
//				Name:      key.Name,
//			}
//		}
//
//		gname := schema.GroupName{
//			Group: e.Group,
//			Kind:  e.Kind,
//			Name:  e.Name,
//		}
//
//		if v, ok := vcache[gname]; ok {
//			e.Version = v
//		}
//		components = append(components, e)
//	}
//
//	// Augment components with children
//	for i := range components {
//		c := components[i]
//		resourceKey := cache.ResourceKeyFromComponentAttributes(c)
//		entry, exists := cache.GetResourceCache().GetCacheEntry(resourceKey.ObjectIdentifier())
//		if !exists {
//			continue
//		}
//
//		children, err := db.GetComponentCache().ComponentChildren(entry.GetUID())
//		if err != nil {
//			klog.ErrorS(err, "failed to get children for component", "component", c.Name)
//			continue
//		}
//
//		components[i].Children = lo.ToSlicePtr(children)
//	}
//
//	return components
//}
