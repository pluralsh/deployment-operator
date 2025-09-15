package service

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pluralsh/deployment-operator/pkg/images"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// Helper function to extract images from applied resources and create metadata
func (s *ServiceReconciler) ExtractImagesMetadata(appliedResources []any) *console.ServiceMetadataAttributes {
	var allImages []string

	for _, resource := range appliedResources {
		if unstructuredObj, ok := resource.(*unstructured.Unstructured); ok {
			if componentImages := images.ExtractImagesFromResource(unstructuredObj); componentImages != nil {
				allImages = append(allImages, componentImages...)
			}
		}
	}

	if len(allImages) == 0 {
		return nil
	}

	uniqueImages := lo.Uniq(allImages)
	return &console.ServiceMetadataAttributes{
		Images: lo.ToSlicePtr(uniqueImages),
	}
}

func (s *ServiceReconciler) UpdateStatus(id, revisionID string, sha *string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes, metadata *console.ServiceMetadataAttributes) error {
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

	return s.consoleClient.UpdateComponents(id, revisionID, sha, components, errs, metadata)
}

func (s *ServiceReconciler) UpdateErrors(id string, err *console.ServiceErrorAttributes) error {
	return s.consoleClient.UpdateServiceErrors(id, lo.Ternary(err != nil, []*console.ServiceErrorAttributes{err}, []*console.ServiceErrorAttributes{}))
}
