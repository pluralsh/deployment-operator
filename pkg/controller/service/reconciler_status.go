package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/print/stats"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
	HealthStatusPaused    HealthStatusCode = "Paused"
	// Degrade status is used if resource status indicates failure or resource could not reach healthy state
	// within some timeout.
	HealthStatusDegraded HealthStatusCode = "Degraded"
	// Indicates that resource is missing in the cluster.
	HealthStatusMissing HealthStatusCode = "Missing"
)

// Represents resource health status
type HealthStatusCode string

// GetResourceHealth returns the health of a k8s resource
func (s *ServiceReconciler) getResourceHealth(obj *unstructured.Unstructured) (health *HealthStatus, err error) {
	if obj.GetDeletionTimestamp() != nil {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Pending deletion",
		}, nil
	}

	if healthCheck := s.GetHealthCheckFunc(obj.GroupVersionKind()); healthCheck != nil {
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
func (s *ServiceReconciler) GetHealthCheckFunc(gvk schema.GroupVersionKind) func(obj *unstructured.Unstructured) (*HealthStatus, error) {
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
		if gvk.Kind == IngressKind {
			return getIngressHealth
		}
	case "networking.k8s.io":
		if gvk.Kind == IngressKind {
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
		if gvk.Kind == JobKind {
			return getJobHealth
		}
	case "flagger.app":
		if gvk.Kind == CanaryKind {
			return getCanaryHealth
		}
	case rollouts.Group:
		if gvk.Kind == rollouts.RolloutKind {
			return getArgoRolloutHealth
		}
	case "autoscaling":
		if gvk.Kind == HorizontalPodAutoscalerKind {
			return getHPAHealth
		}
	}

	if s.GetLuaScript() != "" {
		return s.getLuaHealthConvert
	}

	return getOtherHealth
}

func (s *ServiceReconciler) toStatus(obj *unstructured.Unstructured) *console.ComponentState {
	h, _ := s.getResourceHealth(obj)
	if h == nil {
		return nil
	}

	if h.Status == HealthStatusDegraded {
		return lo.ToPtr(console.ComponentStateFailed)
	}

	if h.Status == HealthStatusHealthy {
		return lo.ToPtr(console.ComponentStateRunning)
	}

	if h.Status == HealthStatusPaused {
		return lo.ToPtr(console.ComponentStatePaused)
	}

	return lo.ToPtr(console.ComponentStatePending)
}

func (s *ServiceReconciler) UpdatePruneStatus(ctx context.Context, svc *console.GetServiceDeploymentForAgent_ServiceDeployment, ch <-chan event.Event, vcache map[manifests.GroupName]string) error {
	logger := log.FromContext(ctx)

	var statsCollector stats.Stats
	var err error
	statusCollector := newServiceComponentsStatusCollector(s, svc)

	for e := range ch {
		statsCollector.Handle(e)
		if e.Type == event.StatusType {
			statusCollector.updateStatus(e.StatusEvent.Identifier, e.StatusEvent)

			gk := e.StatusEvent.Identifier.GroupKind
			name := e.StatusEvent.Identifier.Name
			if e.StatusEvent.Error != nil {
				errorMsg := fmt.Sprintf("%s status %s: %s\n", resourceIDToString(gk, name),
					strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()), e.StatusEvent.Error.Error())
				err = fmt.Errorf(errorMsg)
				logger.Error(err, "status error")
			} else {
				logger.Info(resourceIDToString(gk, name),
					"status", strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()))
			}
		}
	}

	if err := FormatSummary(ctx, svc.Namespace, svc.Name, statsCollector); err != nil {
		return err
	}

	components := statusCollector.componentsAttributes(vcache)
	// delete service when components len == 0 (no new statuses, inventory file is empty, all deleted)
	if err := s.UpdateStatus(svc.ID, components, errorAttributes("sync", err)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
}

func (s *ServiceReconciler) UpdateApplyStatus(ctx context.Context, svc *console.GetServiceDeploymentForAgent_ServiceDeployment, ch <-chan event.Event, printStatus bool, vcache map[manifests.GroupName]string) error {
	logger := log.FromContext(ctx)

	var statsCollector stats.Stats
	var err error
	statusCollector := newServiceComponentsStatusCollector(s, svc)

	for e := range ch {
		statsCollector.Handle(e)
		switch e.Type {
		case event.ActionGroupType:
			if err := FormatActionGroupEvent(ctx, e.ActionGroupEvent); err != nil {
				return err
			}
		case event.ErrorType:
			return e.ErrorEvent.Err
		case event.ApplyType:
			statusCollector.updateApplyStatus(e.ApplyEvent.Identifier, e.ApplyEvent)
			gk := e.ApplyEvent.Identifier.GroupKind
			name := e.ApplyEvent.Identifier.Name
			if e.ApplyEvent.Error != nil {
				msg := fmt.Sprintf("%s apply %s: %s\n", resourceIDToString(gk, name),
					strings.ToLower(e.ApplyEvent.Status.String()), e.ApplyEvent.Error.Error())
				if e.ApplyEvent.Status == event.ApplyFailed {
					err = fmt.Errorf(msg)
					logger.Error(err, "apply error")
				} else {
					logger.Info(msg)
				}
			} else if printStatus {
				logger.Info(resourceIDToString(gk, name),
					"status", strings.ToLower(e.ApplyEvent.Status.String()))
			}

		case event.StatusType:
			statusCollector.updateStatus(e.StatusEvent.Identifier, e.StatusEvent)
			gk := e.StatusEvent.Identifier.GroupKind
			name := e.StatusEvent.Identifier.Name
			if e.StatusEvent.Error != nil {
				errorMsg := fmt.Sprintf("%s status %s: %s\n", resourceIDToString(gk, name),
					strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()), e.StatusEvent.Error.Error())
				err = fmt.Errorf(errorMsg)
				logger.Error(err, "status error")
			} else if printStatus {
				logger.Info(resourceIDToString(gk, name),
					"status", strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()))
			}
		}
	}

	if err := FormatSummary(ctx, svc.Namespace, svc.Name, statsCollector); err != nil {
		return err
	}

	components := statusCollector.componentsAttributes(vcache)
	if err := s.UpdateStatus(svc.ID, components, errorAttributes("sync", err)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
}

func FormatSummary(ctx context.Context, namespace, name string, s stats.Stats) error {
	logger := log.FromContext(ctx)

	if s.ApplyStats != (stats.ApplyStats{}) {
		as := s.ApplyStats
		logger.Info(fmt.Sprintf("apply result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
			namespace, name, as.Sum(), as.Successful, as.Skipped, as.Failed))
	}
	if s.PruneStats != (stats.PruneStats{}) {
		ps := s.PruneStats
		logger.Info(fmt.Sprintf("prune result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
			namespace, name, ps.Sum(), ps.Successful, ps.Skipped, ps.Failed))
	}
	if s.DeleteStats != (stats.DeleteStats{}) {
		ds := s.DeleteStats
		logger.Info(fmt.Sprintf("delete result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
			namespace, name, ds.Sum(), ds.Successful, ds.Skipped, ds.Failed))
	}
	if s.WaitStats != (stats.WaitStats{}) {
		ws := s.WaitStats
		logger.Info(fmt.Sprintf("reconcile result for %s/%s: %d attempted, %d successful, %d skipped, %d failed, %d timed out",
			namespace, name, ws.Sum(), ws.Successful, ws.Skipped, ws.Failed, ws.Timeout))
	}
	return nil
}

func FormatActionGroupEvent(ctx context.Context, age event.ActionGroupEvent) error {
	logger := log.FromContext(ctx)

	switch age.Action {
	case event.ApplyAction:
		logger.V(2).Info("apply phase", "status", strings.ToLower(age.Status.String()))
	case event.PruneAction:
		logger.V(2).Info("prune phase ", "status", strings.ToLower(age.Status.String()))
	case event.DeleteAction:
		logger.V(2).Info("delete phase", "status", strings.ToLower(age.Status.String()))
	case event.WaitAction:
		logger.V(2).Info("reconcile phase", "status", strings.ToLower(age.Status.String()))
	case event.InventoryAction:
		logger.V(2).Info("inventory update", "status", strings.ToLower(age.Status.String()))
	default:
		return fmt.Errorf("invalid action group action: %+v", age)
	}
	return nil
}

func (s *ServiceReconciler) UpdateErrorStatus(ctx context.Context, id string, err error) {
	logger := log.FromContext(ctx)

	if err := s.AddErrors(id, errorAttributes("sync", err)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}
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

func (s *ServiceReconciler) UpdateStatus(id string, components []*console.ComponentAttributes, err *console.ServiceErrorAttributes) error {
	errs := make([]*console.ServiceErrorAttributes, 0)
	if err != nil {
		errs = append(errs, err)
	}

	return s.ConsoleClient.UpdateComponents(id, components, errs)
}

func (s *ServiceReconciler) AddErrors(id string, err *console.ServiceErrorAttributes) error {
	return s.ConsoleClient.AddServiceErrors(id, []*console.ServiceErrorAttributes{err})
}

func resourceIDToString(gk schema.GroupKind, name string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(gk.String()), name)
}
