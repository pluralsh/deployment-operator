package sync

import (
	"fmt"
	"strings"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/cli-utils/pkg/print/stats"
)

// Represents resource health status
type HealthStatusCode string

const (
	// Indicates that health assessment failed and actual health status is unknown
	HealthStatusUnknown HealthStatusCode = "Unknown"
	// Progressing health status means that resource is not healthy but still have a chance to reach healthy state
	HealthStatusProgressing HealthStatusCode = "Progressing"
	// Resource is 100% healthy
	HealthStatusHealthy HealthStatusCode = "Healthy"
	// Assigned to resources that are suspended or paused. The typical example is a
	// [suspended](https://kubernetes.io/docs/tasks/job/automated-tasks-with-cron-jobs/#suspend) CronJob.
	HealthStatusSuspended HealthStatusCode = "Suspended"
	// Degrade status is used if resource status indicates failure or resource could not reach healthy state
	// within some timeout.
	HealthStatusDegraded HealthStatusCode = "Degraded"
	// Indicates that resource is missing in the cluster.
	HealthStatusMissing HealthStatusCode = "Missing"
)

type HealthStatus struct {
	Status  HealthStatusCode `json:"status,omitempty"`
	Message string           `json:"message,omitempty"`
}

// GetResourceHealth returns the health of a k8s resource
func getResourceHealth(obj *unstructured.Unstructured) (health *HealthStatus, err error) {
	if obj.GetDeletionTimestamp() != nil {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Pending deletion",
		}, nil
	}

	if healthCheck := GetHealthCheckFunc(obj.GroupVersionKind()); healthCheck != nil {
		if health, err = healthCheck(obj); err != nil {
			health = &HealthStatus{
				Status:  HealthStatusUnknown,
				Message: err.Error(),
			}
		}
	}
	return health, err

}

// GetHealthCheckFunc returns built-in health check function or nil if health check is not supported
func GetHealthCheckFunc(gvk schema.GroupVersionKind) func(obj *unstructured.Unstructured) (*HealthStatus, error) {
	switch gvk.Group {
	case "apps":
		switch gvk.Kind {
		case DeploymentKind:
			return getDeploymentHealth
		case StatefulSetKind:
			return getStatefulSetHealth
		case ReplicaSetKind:
			return getReplicaSetHealth
		case DaemonSetKind:
			return getDaemonSetHealth
		}
	case "extensions":
		switch gvk.Kind {
		case IngressKind:
			return getIngressHealth
		}
	case "networking.k8s.io":
		switch gvk.Kind {
		case IngressKind:
			return getIngressHealth
		}
	case "":
		switch gvk.Kind {
		case ServiceKind:
			return getServiceHealth
		case PersistentVolumeClaimKind:
			return getPVCHealth
		case PodKind:
			return getPodHealth
		}
	case "batch":
		switch gvk.Kind {
		case JobKind:
			return getJobHealth
		}
	case "autoscaling":
		switch gvk.Kind {
		case HorizontalPodAutoscalerKind:
			return getHPAHealth
		}
	}
	return nil
}

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
		State:     toStatus(e.Resource),
	}
}

func toStatus(obj *unstructured.Unstructured) *console.ComponentState {
	h, _ := getResourceHealth(obj)
	if h == nil {
		return nil
	}

	if h.Status == HealthStatusDegraded {
		return lo.ToPtr(console.ComponentStateFailed)
	}

	if h.Status == HealthStatusHealthy {
		return lo.ToPtr(console.ComponentStateRunning)
	}

	return lo.ToPtr(console.ComponentStatePending)
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
