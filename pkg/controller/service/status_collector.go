package service

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type serviceComponentsStatusCollector struct {
	latestStatus     map[object.ObjMetadata]event.StatusEvent
	reconciler       *ServiceReconciler
	applyStatus      map[object.ObjMetadata]event.ApplyEvent
	componentContent map[object.ObjMetadata]console.ComponentContentAttributes
	DryRun           bool
}

func newServiceComponentsStatusCollector(reconciler *ServiceReconciler, svc *console.ServiceDeploymentExtended) *serviceComponentsStatusCollector {
	return &serviceComponentsStatusCollector{
		latestStatus:     make(map[object.ObjMetadata]event.StatusEvent),
		reconciler:       reconciler,
		applyStatus:      make(map[object.ObjMetadata]event.ApplyEvent),
		componentContent: getComponentContent(svc),
		DryRun:           *svc.DryRun,
	}
}

func (sc *serviceComponentsStatusCollector) updateStatus(id object.ObjMetadata, se event.StatusEvent) {
	sc.latestStatus[id] = se
}

func (sc *serviceComponentsStatusCollector) updateApplyStatus(id object.ObjMetadata, se event.ApplyEvent) {
	sc.applyStatus[id] = se
}

func (sc *serviceComponentsStatusCollector) componentsAttributes(vcache map[manifests.GroupName]string) []*console.ComponentAttributes {
	components := make([]*console.ComponentAttributes, 0, len(sc.latestStatus))
	for _, v := range sc.latestStatus {
		if attrs := sc.componentAttributes(v, vcache); attrs != nil {
			components = append(components, attrs)
		}
	}

	return components
}

func (sc *serviceComponentsStatusCollector) componentAttributes(e event.StatusEvent, vcache map[manifests.GroupName]string) *console.ComponentAttributes {
	if e.Resource == nil {
		return nil
	}

	gvk := e.Resource.GroupVersionKind()
	gname := manifests.GroupName{
		Group: gvk.Group,
		Kind:  gvk.Kind,
		Name:  e.Resource.GetName(),
	}

	version := gvk.Version
	if v, ok := vcache[gname]; ok {
		version = v
	}

	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    e.PollResourceInfo.Status == status.CurrentStatus,
		State:     sc.reconciler.toStatus(e.Resource),
	}
}

func getComponentContent(svc *console.ServiceDeploymentExtended) map[object.ObjMetadata]console.ComponentContentAttributes {
	result := make(map[object.ObjMetadata]console.ComponentContentAttributes)

	for _, comp := range svc.Components {
		namespace := ""
		group := ""
		if comp.Namespace != nil {
			namespace = *comp.Namespace
		}
		if comp.Group != nil {
			group = *comp.Group
		}
		gn := object.ObjMetadata{
			Namespace: namespace,
			Name:      comp.Name,
			GroupKind: schema.GroupKind{
				Group: group,
				Kind:  comp.Kind,
			},
		}
		if comp.Content != nil {
			result[gn] = console.ComponentContentAttributes{
				Desired: comp.Content.Desired,
				Live:    comp.Content.Live,
			}
		}
	}

	return result
}
