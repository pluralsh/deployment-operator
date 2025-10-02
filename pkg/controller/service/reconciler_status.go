package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	plrlog "github.com/pluralsh/deployment-operator/pkg/log"
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

func (s *ServiceReconciler) UpdateStatus(ctx context.Context, id, revisionID string, sha *string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error {
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
			slices.SortFunc(component.Children, func(a, b *console.ComponentChildAttributes) int {
				return strings.Compare(componentChildKey(*a), componentChildKey(*b))
			})
		}
	}

	slices.SortFunc(components, func(a, b *console.ComponentAttributes) int {
		return strings.Compare(componentKey(*a), componentKey(*b))
	})

	// hash the components and errors to determine if there has been a meaningful change
	// we need to report to the server
	objToHash := struct {
		Components []*console.ComponentAttributes    `json:"components"`
		Errs       []*console.ServiceErrorAttributes `json:"errs"`
		RevisionID string                            `json:"revisionId"`
		Sha        *string                           `json:"sha,omitempty"`
	}{
		Components: components,
		Errs:       errs,
		RevisionID: revisionID,
		Sha:        sha,
	}

	hashedSha, err := utils.HashObject(objToHash)
	if err != nil {
		log.Log.Error(err, "Failed to hash service components")
		hashedSha = "__ignore__"
	}

	logger := log.FromContext(ctx).V(int(plrlog.LogLevelDefault))
	shaCache := cache.ComponentShaCache()

	old, found := shaCache.Get(id)
	if found && old == hashedSha {
		logger.Info("No meaningful change in components, skipping update to console api")
		return nil
	}

	if hashedSha != "__ignore__" {
		shaCache.Add(id, hashedSha)
	}

	return s.consoleClient.UpdateComponents(id, revisionID, sha, components, errs)
}

func componentChildKey(c console.ComponentChildAttributes) string {
	group, ns := "", ""
	if c.Group != nil {
		group = *c.Group
	}

	if c.Namespace != nil {
		ns = *c.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s", group, c.Version, c.Kind, c.Name, ns)
}

func componentKey(c console.ComponentAttributes) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s", c.Group, c.Version, c.Kind, c.Name, c.Namespace)
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
