package statusreaders

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/engine"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
	"sigs.k8s.io/cli-utils/pkg/object"
)

// baseStatusReader is the implementation of the StatusReader interface defined
// in the engine package. It contains the basic logic needed for every resource.
// In order to handle resource specific logic, it must include an implementation
// of the resourceTypeStatusReader interface.
// In practice we will create many instances of baseStatusReader, each with a different
// implementation of the resourceTypeStatusReader interface and therefore each
// of the instances will be able to handle different resource types.
type baseStatusReader struct {
	// mapper provides a way to look up the resource types that are available
	// in the cluster.
	mapper meta.RESTMapper

	// resourceStatusReader is an resource-type specific implementation
	// of the resourceTypeStatusReader interface. While the baseStatusReader
	// contains the logic shared between all resource types, this implementation
	// will contain the resource specific info.
	resourceStatusReader resourceTypeStatusReader
}

// resourceTypeStatusReader is an interface that can be implemented differently
// for each resource type.
type resourceTypeStatusReader interface {
	Supports(gk schema.GroupKind) bool
	ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, object *unstructured.Unstructured) (*event.ResourceStatus, error)
}

func (b *baseStatusReader) Supports(gk schema.GroupKind) bool {
	return b.resourceStatusReader.Supports(gk)
}

// ReadStatus reads the object identified by the passed-in identifier and computes it's status. It reads
// the resource here, but computing status is delegated to the ReadStatusForObject function.
func (b *baseStatusReader) ReadStatus(ctx context.Context, reader engine.ClusterReader, identifier object.ObjMetadata) (*event.ResourceStatus, error) {
	object, err := b.lookupResource(ctx, reader, identifier)
	if err != nil {
		return errIdentifierToResourceStatus(err, identifier)
	}
	return b.resourceStatusReader.ReadStatusForObject(ctx, reader, object)
}

// ReadStatusForObject computes the status for the passed-in object. Since this is specific for each
// resource type, the actual work is delegated to the implementation of the resourceTypeStatusReader interface.
func (b *baseStatusReader) ReadStatusForObject(ctx context.Context, reader engine.ClusterReader, object *unstructured.Unstructured) (*event.ResourceStatus, error) {
	return b.resourceStatusReader.ReadStatusForObject(ctx, reader, object)
}

// lookupResource looks up a resource with the given identifier. It will use the rest mapper to resolve
// the version of the GroupKind given in the identifier.
// If the resource is found, it is returned. If it is not found or something
// went wrong, the function will return an error.
func (b *baseStatusReader) lookupResource(ctx context.Context, reader engine.ClusterReader, identifier object.ObjMetadata) (*unstructured.Unstructured, error) {
	GVK, err := gvk(identifier.GroupKind, b.mapper)
	if err != nil {
		return nil, err
	}

	var u unstructured.Unstructured
	u.SetGroupVersionKind(GVK)
	key := types.NamespacedName{
		Name:      identifier.Name,
		Namespace: identifier.Namespace,
	}
	err = reader.Get(ctx, key, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// gvk looks up the GVK from a GroupKind using the rest mapper.
func gvk(gk schema.GroupKind, mapper meta.RESTMapper) (schema.GroupVersionKind, error) {
	mapping, err := mapper.RESTMapping(gk)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return mapping.GroupVersionKind, nil
}

// errResourceToResourceStatus construct the appropriate ResourceStatus
// object based on an error and the resource itself.
func errResourceToResourceStatus(err error, resource *unstructured.Unstructured, genResources ...*event.ResourceStatus) (*event.ResourceStatus, error) {
	// If the error is from the context, we don't attach that to the ResourceStatus,
	// but just return it directly so the caller can decide how to handle this
	// situation.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	identifier := object.UnstructuredToObjMetadata(resource)
	if apierrors.IsNotFound(err) {
		return &event.ResourceStatus{
			Identifier: identifier,
			Status:     status.NotFoundStatus,
			Message:    "Resource not found",
		}, nil
	}
	return &event.ResourceStatus{
		Identifier:         identifier,
		Status:             status.UnknownStatus,
		Resource:           resource,
		Error:              err,
		GeneratedResources: genResources,
	}, nil
}

// errIdentifierToResourceStatus construct the appropriate ResourceStatus
// object based on an error and the identifier for a resource.
func errIdentifierToResourceStatus(err error, identifier object.ObjMetadata) (*event.ResourceStatus, error) {
	// If the error is from the context, we don't attach that to the ResourceStatus,
	// but just return it directly so the caller can decide how to handle this
	// situation.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		return &event.ResourceStatus{
			Identifier: identifier,
			Status:     status.NotFoundStatus,
			Message:    "Resource not found",
		}, nil
	}
	return &event.ResourceStatus{
		Identifier: identifier,
		Status:     status.UnknownStatus,
		Error:      err,
	}, nil
}
