package cache

import (
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const resourceKeyPlaceholder = "*"

type ResourceKey object.ObjMetadata

// String returns string representation of ResourceKey.
// Name and namespace are replaced with placeholders as they cannot be empty to parse it back from the string.
func (in ResourceKey) String() string {
	in.Name = resourceKeyPlaceholder
	in.Namespace = resourceKeyPlaceholder

	return in.String()
}

func ObjMetadataSetToResourceKeys(set object.ObjMetadataSet) containers.Set[string] {
	return containers.ToSet(algorithms.Map(set, func(obj object.ObjMetadata) string {
		return ResourceKey(obj).String()
	}))
}

func UnstructuredServiceDeploymentToID(obj *unstructured.Unstructured) string {
	id := obj.GetAnnotations()[inventory.OwningInventoryKey]
	return id
}
