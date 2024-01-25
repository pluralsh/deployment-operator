package service

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type StatusCollector struct {
	latestStatus map[object.ObjMetadata]event.StatusEvent
	reconciler   *ServiceReconciler
}

func (sc *StatusCollector) updateStatus(id object.ObjMetadata, se event.StatusEvent) {
	sc.latestStatus[id] = se
}

func (sc *StatusCollector) Components(vcache map[manifests.GroupName]string) []*console.ComponentAttributes {
	components := []*console.ComponentAttributes{}
	for _, v := range sc.latestStatus {
		consoleAttr := sc.reconciler.FromSyncResult(v, vcache)
		if consoleAttr != nil {
			components = append(components, consoleAttr)
		}
	}

	return components
}
