package service

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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

func (s *ServiceReconciler) UpdateStatus(id, revisionID string, sha *string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error {
	for _, component := range components {
		if component.State != nil && *component.State == console.ComponentStateRunning {
			// Skip checking child pods for the Job. The database cache contains only failed pods, and the Job may succeed after a retry.
			if component.Kind == "Job" {
				continue
			}
			for _, child := range component.Children {
				if child.State != nil && *child.State != console.ComponentStateRunning {
					component.State = child.State
					break
				}
			}
		}
	}

	return s.consoleClient.UpdateComponents(id, revisionID, sha, components, errs)
}

func (s *ServiceReconciler) UpdateErrors(id string, err *console.ServiceErrorAttributes) error {
	return s.consoleClient.UpdateServiceErrors(id, lo.Ternary(err != nil, []*console.ServiceErrorAttributes{err}, []*console.ServiceErrorAttributes{}))
}
