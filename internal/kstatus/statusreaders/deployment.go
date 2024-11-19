package statusreaders

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

func NewDeploymentResourceReader(mapper meta.RESTMapper) engine.StatusReader {
	return &baseStatusReader{
		mapper: mapper,
		resourceStatusReader: &deploymentResourceReader{
			mapper: mapper,
		},
	}
}

// deploymentResourceReader is a resourceTypeStatusReader that can fetch Deployment
// resources from the cluster, knows how to find any ReplicaSets belonging to the
// Deployment, and compute status for the deployment.
type deploymentResourceReader struct {
	mapper meta.RESTMapper
}

var _ resourceTypeStatusReader = &deploymentResourceReader{}

func (d *deploymentResourceReader) Supports(gk schema.GroupKind) bool {
	return gk == appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind()
}

func (d *deploymentResourceReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader,
	deployment *unstructured.Unstructured) (*event.ResourceStatus, error) {
	identifier := object.UnstructuredToObjMetadata(deployment)

	res, err := status.Compute(deployment)
	if err != nil {
		return errResourceToResourceStatus(err, deployment, []*event.ResourceStatus{}...)
	}

	return &event.ResourceStatus{
		Identifier:         identifier,
		Status:             res.Status,
		Resource:           deployment,
		Message:            res.Message,
		GeneratedResources: []*event.ResourceStatus{},
	}, nil
}
