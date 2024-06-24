package cache

import (
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
)

const resourceKeyPlaceholder = "*"

func ObjMetadataSetToResourceKeys(set object.ObjMetadataSet) containers.Set[string] {
	return containers.ToSet(algorithms.Map(set, func(obj object.ObjMetadata) string {
		return ObjMetadataToResourceKey(obj)
	}))
}

func ObjMetadataToResourceKey(obj object.ObjMetadata) string {
	// name/namespace in ObjMetadata cannot be empty in order to parse it back from the string.

	obj.Name = resourceKeyPlaceholder
	obj.Namespace = resourceKeyPlaceholder

	return obj.String()
}

func UnstructuredServiceDeploymentToID(obj *unstructured.Unstructured) string {
	id := obj.GetAnnotations()[inventory.OwningInventoryKey]
	return id
}
