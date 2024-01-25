package service

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type StatusCollector struct {
	latestStatus map[object.ObjMetadata]event.StatusEvent
	reconciler   *ServiceReconciler
}

func newStatusCollector(reconciler *ServiceReconciler) *StatusCollector {
	return &StatusCollector{
		latestStatus: make(map[object.ObjMetadata]event.StatusEvent),
		reconciler:   reconciler,
	}
}

func (sc *StatusCollector) updateStatus(id object.ObjMetadata, se event.StatusEvent) {
	sc.latestStatus[id] = se
}

func (sc *StatusCollector) components(vcache map[manifests.GroupName]string) []*console.ComponentAttributes {
	components := make([]*console.ComponentAttributes, 0, len(sc.latestStatus))
	for _, v := range sc.latestStatus {
		if attrs := sc.component(v, vcache); attrs != nil {
			components = append(components, attrs)
		}
	}

	return components
}

func (sc *StatusCollector) component(e event.StatusEvent, vcache map[manifests.GroupName]string) *console.ComponentAttributes {
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
