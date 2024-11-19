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

func NewStatefulSetResourceReader(mapper meta.RESTMapper) engine.StatusReader {
	return &baseStatusReader{
		mapper: mapper,
		resourceStatusReader: &statefulSetResourceReader{
			mapper: mapper,
		},
	}
}

// statefulSetResourceReader is an implementation of the ResourceReader interface
// that can fetch StatefulSet resources from the cluster, knows how to find any
// Pods belonging to the StatefulSet, and compute status for the StatefulSet.
type statefulSetResourceReader struct {
	mapper meta.RESTMapper
}

var _ resourceTypeStatusReader = &statefulSetResourceReader{}

func (s *statefulSetResourceReader) Supports(gk schema.GroupKind) bool {
	return gk == appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind()
}

func (s *statefulSetResourceReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader,
	statefulSet *unstructured.Unstructured) (*event.ResourceStatus, error) {
	identifier := object.UnstructuredToObjMetadata(statefulSet)
	res, err := status.Compute(statefulSet)
	if err != nil {
		return errResourceToResourceStatus(err, statefulSet, []*event.ResourceStatus{}...)
	}

	return &event.ResourceStatus{
		Identifier:         identifier,
		Status:             res.Status,
		Resource:           statefulSet,
		Message:            res.Message,
		GeneratedResources: []*event.ResourceStatus{},
	}, nil
}
