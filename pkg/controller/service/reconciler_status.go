package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	"sigs.k8s.io/cli-utils/pkg/print/stats"
	"sigs.k8s.io/controller-runtime/pkg/log"

	internalschema "github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

func (s *ServiceReconciler) UpdatePruneStatus(
	ctx context.Context,
	svc *console.ServiceDeploymentForAgent,
	ch <-chan event.Event,
	vcache map[internalschema.GroupName]string,
) error {
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
				err = fmt.Errorf("%s status %s: %s", resourceIDToString(gk, name),
					strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()), e.StatusEvent.Error.Error())
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
	if err := s.UpdateStatus(svc.ID, svc.Revision.ID, svc.Sha, components, errorAttributes("sync", err)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}

	return nil
}

func (s *ServiceReconciler) UpdateApplyStatus(
	ctx context.Context,
	svc *console.ServiceDeploymentForAgent,
	ch <-chan event.Event,
	printStatus bool,
	vcache map[internalschema.GroupName]string,
) error {
	logger := log.FromContext(ctx)
	start, err := metrics.FromContext[time.Time](ctx, metrics.ContextKeyTimeStart)
	if err != nil {
		klog.Fatalf("programmatic error! context does not have value for the key %s", metrics.ContextKeyTimeStart)
	}

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationApplyStart),
	)

	var statsCollector stats.Stats
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
			namespace := e.ApplyEvent.Identifier.Namespace
			if e.ApplyEvent.Status == event.ApplySuccessful {
				cache.SaveResourceSHA(e.ApplyEvent.Resource, cache.ApplySHA)
			}
			if e.ApplyEvent.Error != nil {
				if e.ApplyEvent.Status == event.ApplyFailed {
					// e.ApplyEvent.Resource == nil, create the key to get cache entry
					key := cache.ResourceKey{
						Namespace: namespace,
						Name:      name,
						GroupKind: gk,
					}
					sha, exists := cache.GetResourceCache().GetCacheEntry(key.ObjectIdentifier())
					if exists {
						// clear SHA when error occurs
						sha.Expire()
						cache.GetResourceCache().SetCacheEntry(key.ObjectIdentifier(), sha)
					}
					err = fmt.Errorf("%s apply %s: %s", resourceIDToString(gk, name),
						strings.ToLower(e.ApplyEvent.Status.String()), e.ApplyEvent.Error.Error())
					logger.Error(err, "apply error")
				} else {
					msg := fmt.Sprintf("%s apply %s: %s\n", resourceIDToString(gk, name),
						strings.ToLower(e.ApplyEvent.Status.String()), e.ApplyEvent.Error.Error())
					logger.V(4).Info(msg)
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
				err = fmt.Errorf("%s status %s: %s", resourceIDToString(gk, name),
					strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()), e.StatusEvent.Error.Error())
				logger.Error(err, "status error")
			} else if printStatus {
				logger.Info(resourceIDToString(gk, name),
					"status", strings.ToLower(e.StatusEvent.PollResourceInfo.Status.String()))
			}
		}
	}

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationApplyFinish),
	)

	if err := FormatSummary(ctx, svc.Namespace, svc.Name, statsCollector); err != nil {
		return err
	}
	components := statusCollector.componentsAttributes(vcache)
	if err := s.UpdateStatus(svc.ID, svc.Revision.ID, svc.Sha, components, errorAttributes("sync", err)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	}

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationUpdateStatusFinish),
	)

	return nil
}

func FormatSummary(ctx context.Context, namespace, name string, s stats.Stats) error {
	logger := log.FromContext(ctx)

	if s.ApplyStats != (stats.ApplyStats{}) {
		as := s.ApplyStats
		logger.V(4).Info(fmt.Sprintf("apply result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
			namespace, name, as.Sum(), as.Successful, as.Skipped, as.Failed))
	}
	if s.PruneStats != (stats.PruneStats{}) {
		ps := s.PruneStats
		logger.V(4).Info(fmt.Sprintf("prune result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
			namespace, name, ps.Sum(), ps.Successful, ps.Skipped, ps.Failed))
	}
	if s.DeleteStats != (stats.DeleteStats{}) {
		ds := s.DeleteStats
		logger.V(4).Info(fmt.Sprintf("delete result for %s/%s: %d attempted, %d successful, %d skipped, %d failed",
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
	if err := s.UpdateErrors(id, errorAttributes("sync", err)); err != nil {
		log.FromContext(ctx).Error(err, "Failed to update service status, ignoring for now")
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

func (s *ServiceReconciler) UpdateStatus(id, revisionID string, sha *string, components []*console.ComponentAttributes, err *console.ServiceErrorAttributes) error {
	errs := make([]*console.ServiceErrorAttributes, 0)
	if err != nil {
		errs = append(errs, err)
	}

	return s.consoleClient.UpdateComponents(id, revisionID, sha, components, errs)
}

func (s *ServiceReconciler) UpdateErrors(id string, err *console.ServiceErrorAttributes) error {
	return s.consoleClient.UpdateServiceErrors(id, []*console.ServiceErrorAttributes{err})
}

func resourceIDToString(gk schema.GroupKind, name string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(gk.String()), name)
}
