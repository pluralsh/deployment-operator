package service

import (
	"context"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"

	"github.com/pluralsh/deployment-operator/pkg/manifests"
)

type serviceComponentsStatusCollector struct {
	latestStatus map[object.ObjMetadata]event.StatusEvent
	reconciler   *ServiceReconciler
	applyStatus  map[object.ObjMetadata]event.ApplyEvent
	DryRun       bool
}

func newServiceComponentsStatusCollector(reconciler *ServiceReconciler, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) *serviceComponentsStatusCollector {
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
	if sc.reconciler.Clientset == nil || resource == nil {
		return nil
	}

	response := new(unstructured.Unstructured)
	err := sc.reconciler.Clientset.RESTClient().Get().AbsPath(toAPIPath(resource)).Do(context.Background()).Into(response)
	if err != nil {
		return nil
	}

	return response
}

func (sc *serviceComponentsStatusCollector) fromApplyResult(e event.ApplyEvent, vcache map[manifests.GroupName]string) *console.ComponentAttributes {
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

	desired := asJSON(e.Resource)
	live := "# n/a"
	liveResource := sc.refetch(e.Resource)
	if liveResource != nil {
		live = asJSON(liveResource)
	}

	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    live == desired,
		State:     sc.reconciler.toStatus(e.Resource),
		Content: &console.ComponentContentAttributes{
			Desired: &desired,
			Live:    &live,
		},
	}
}

func (sc *serviceComponentsStatusCollector) fromSyncResult(e event.StatusEvent, vcache map[manifests.GroupName]string) *console.ComponentAttributes {
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

	synced := e.PollResourceInfo.Status == status.CurrentStatus

	if e.PollResourceInfo.Status == status.UnknownStatus {
		if sc.reconciler.toStatus(e.Resource) != nil {
			synced = *sc.reconciler.toStatus(e.Resource) == console.ComponentStateRunning
		}
	}
	return &console.ComponentAttributes{
		Group:     gvk.Group,
		Kind:      gvk.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   version,
		Synced:    synced,
		State:     sc.reconciler.toStatus(e.Resource),
	}
}

func (sc *serviceComponentsStatusCollector) componentsAttributes(vcache map[manifests.GroupName]string) []*console.ComponentAttributes {
	components := make([]*console.ComponentAttributes, 0, len(sc.latestStatus))

	if sc.DryRun {
		for _, v := range sc.applyStatus {
			if consoleAttr := sc.fromApplyResult(v, vcache); consoleAttr != nil {
				components = append(components, consoleAttr)
			}
		}
		return components
	}

	for _, v := range sc.latestStatus {
		if attrs := sc.fromSyncResult(v, vcache); attrs != nil {
			components = append(components, attrs)
		}
	}

	return components
}
