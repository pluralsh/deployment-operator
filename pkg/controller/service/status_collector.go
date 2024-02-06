package service

import (
	"sigs.k8s.io/yaml"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/samber/lo"
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
	if svc.DryRun == nil {
		svc.DryRun = lo.ToPtr(false)
	}
	return &serviceComponentsStatusCollector{
		latestStatus:     make(map[object.ObjMetadata]event.StatusEvent),
		applyStatus:      make(map[object.ObjMetadata]event.ApplyEvent),
		reconciler:       reconciler,
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

func (sc *serviceComponentsStatusCollector) fromApplyResult(e event.ApplyEvent, vcache map[manifests.GroupName]string, compCont map[object.ObjMetadata]console.ComponentContentAttributes) *console.ComponentAttributes {
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

	live := "# n/a"
	if compCont != nil {
		compName := object.ObjMetadata{
			Namespace: e.Resource.GetNamespace(),
			Name:      e.Resource.GetName(),
			GroupKind: gvk.GroupKind(),
		}
		if r, ok := compCont[compName]; ok {
			if r.Live != nil {
				live = *r.Live
			}
		}
	}

	desiredData, _ := yaml.Marshal(e.Resource.Object)
	desired := string(desiredData)

	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    true,
		State:     sc.reconciler.toStatus(e.Resource),
		Content: &console.ComponentContentAttributes{
			Desired: &desired,
			Live:    &live,
		},
	}
}

func (sc *serviceComponentsStatusCollector) fromSyncResult(e event.StatusEvent, vcache map[manifests.GroupName]string, compCont map[object.ObjMetadata]console.ComponentContentAttributes) *console.ComponentAttributes {
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

	desired := "# n/a"
	if compCont != nil {
		compName := object.ObjMetadata{
			Namespace: e.Resource.GetNamespace(),
			Name:      e.Resource.GetName(),
			GroupKind: gvk.GroupKind(),
		}
		if r, ok := compCont[compName]; ok {
			if r.Desired != nil {
				desired = *r.Desired
			}
		}
	}

	liveData, _ := yaml.Marshal(e.Resource.Object)
	live := string(liveData)

	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    e.PollResourceInfo.Status == status.CurrentStatus,
		State:     sc.reconciler.toStatus(e.Resource),
		Content: &console.ComponentContentAttributes{
			Live:    &live,
			Desired: &desired,
		},
	}
}

func (sc *serviceComponentsStatusCollector) componentsAttributes(vcache map[manifests.GroupName]string) []*console.ComponentAttributes {
	components := make([]*console.ComponentAttributes, 0, len(sc.latestStatus))

	if sc.DryRun {
		for _, v := range sc.applyStatus {
			if consoleAttr := sc.fromApplyResult(v, vcache, sc.componentContent); consoleAttr != nil {
				components = append(components, consoleAttr)
			}
		}
		return components
	}

	for _, v := range sc.latestStatus {
		if attrs := sc.fromSyncResult(v, vcache, sc.componentContent); attrs != nil {
			components = append(components, attrs)
		}
	}

	return components
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
