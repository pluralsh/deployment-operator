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

func NewReplicaSetStatusReader(mapper meta.RESTMapper) engine.StatusReader {
	return &baseStatusReader{
		mapper: mapper,
		resourceStatusReader: &replicaSetStatusReader{
			mapper: mapper,
		},
	}
}

// replicaSetStatusReader is an engine that can fetch ReplicaSet resources
// from the cluster, knows how to find any Pods belonging to the ReplicaSet,
// and compute status for the ReplicaSet.
type replicaSetStatusReader struct {
	mapper meta.RESTMapper
}

var _ resourceTypeStatusReader = &replicaSetStatusReader{}

func (r *replicaSetStatusReader) Supports(gk schema.GroupKind) bool {
	return gk == appsv1.SchemeGroupVersion.WithKind("ReplicaSet").GroupKind()
}

func (r *replicaSetStatusReader) ReadStatusForObject(_ context.Context, _ engine.ClusterReader, rs *unstructured.Unstructured) (*event.ResourceStatus, error) {
	identifier := object.UnstructuredToObjMetadata(rs)
	res, err := status.Compute(rs)
	if err != nil {
		return errResourceToResourceStatus(err, rs, []*event.ResourceStatus{}...)
	}

	return &event.ResourceStatus{
		Identifier:         identifier,
		Status:             res.Status,
		Resource:           rs,
		Message:            res.Message,
		GeneratedResources: []*event.ResourceStatus{},
	}, nil
}
