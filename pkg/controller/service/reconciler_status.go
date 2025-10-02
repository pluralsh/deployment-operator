package service

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/images"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
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

func (s *ServiceReconciler) ExtractImagesMetadata(manifests []unstructured.Unstructured) *console.ServiceMetadataAttributes {
	var allImages []string

	klog.Infof("Extracting images from %d manifests", len(manifests))

	for i, resource := range manifests {
		klog.Infof("Processing manifest %d: %s %s/%s", i+1, resource.GetKind(), resource.GetNamespace(), resource.GetName())
		if componentImages := images.ExtractImagesFromResource(&resource); componentImages != nil {
			klog.Infof("Found %d images in manifest %d: %v", len(componentImages), i+1, componentImages)
			allImages = append(allImages, componentImages...)
		} else {
			klog.Infof("No images found in manifest %d", i+1)
		}
	}

	if len(allImages) == 0 {
		klog.Info("No images found in any manifests")
		return nil
	}

	uniqueImages := lo.Uniq(allImages)
	klog.Infof("Extracted %d unique images: %v", len(uniqueImages), uniqueImages)
	return &console.ServiceMetadataAttributes{
		Images: lo.ToSlicePtr(uniqueImages),
	}
}
