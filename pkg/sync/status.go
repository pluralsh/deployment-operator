package sync

import (
	"fmt"
	"strings"

	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/print/stats"
)

func (engine *Engine) UpdateStatus(id string, ch <-chan event.Event, printStatus bool) error {
	var statsCollector stats.Stats
	var err error
	components := []*console.ComponentAttributes{}
	statusCollector := &StatusCollector{
		latestStatus: make(map[object.ObjMetadata]event.StatusEvent),
	}

	for e := range ch {
		statsCollector.Handle(e)
		switch e.Type {
		case event.ApplyType:
			gk := e.ApplyEvent.Identifier.GroupKind
			name := e.ApplyEvent.Identifier.Name
			if e.ApplyEvent.Error != nil {
				msg := fmt.Sprintf("%s apply %s: %s\n", resourceIDToString(gk, name),
					strings.ToLower(e.ApplyEvent.Status.String()), e.ApplyEvent.Error.Error())
				if e.ApplyEvent.Status == event.ApplyFailed {
					err = fmt.Errorf(msg)
					log.Error(err, "apply error")
				} else {
					log.Info(msg)
				}
			} else {
				log.Info(resourceIDToString(gk, name),
					"status", strings.ToLower(e.ApplyEvent.Status.String()))
			}
		case event.StatusType:
			statusCollector.updateStatus(e.StatusEvent.Identifier, e.StatusEvent)
			if printStatus {
				gk := e.StatusEvent.Identifier.GroupKind
				name := e.StatusEvent.Identifier.Name
				if e.StatusEvent.Error != nil {
					errorMsg := fmt.Sprintf("%s status %s: %s\n", resourceIDToString(gk, name),
						strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()), e.StatusEvent.Error.Error())
					err = fmt.Errorf(errorMsg)
					log.Error(err, "status error")
				} else {
					log.Info(resourceIDToString(gk, name),
						"status", strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()))
				}
			}
		}
	}

	for _, v := range statusCollector.latestStatus {
		consoleAttr := fromSyncResult(v)
		components = append(components, consoleAttr)
	}

	if err := engine.updateStatus(id, components, errorAttributes("sync", err)); err != nil {
		log.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
}

func fromSyncResult(e event.StatusEvent) *console.ComponentAttributes {
	return &console.ComponentAttributes{
		Group:     e.Identifier.GroupKind.Group,
		Kind:      e.Identifier.GroupKind.Kind,
		Namespace: e.Resource.GetNamespace(),
		Name:      e.Resource.GetName(),
		Version:   e.Resource.GetAPIVersion(),
		Synced:    e.PollResourceInfo.Status == status.CurrentStatus,
	}
}

func (engine *Engine) updateStatus(id string, components []*console.ComponentAttributes, err *console.ServiceErrorAttributes) error {
	errs := make([]*console.ServiceErrorAttributes, 0)
	if err != nil {
		errs = append(errs, err)
	}

	return engine.client.UpdateComponents(id, components, errs)
}

func errorAttributes(source string, err error) *console.ServiceErrorAttributes {
	if err == nil {
		return nil
	}

	return &console.ServiceErrorAttributes{
		Source:  source,
		Message: err.Error(),
	}
}

type StatusCollector struct {
	latestStatus map[object.ObjMetadata]event.StatusEvent
}

func (sc *StatusCollector) updateStatus(id object.ObjMetadata, se event.StatusEvent) {
	sc.latestStatus[id] = se
}
func resourceIDToString(gk schema.GroupKind, name string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(gk.String()), name)
}
