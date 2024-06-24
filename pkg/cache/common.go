package cache

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

func UnstructuredServiceDeploymentToID(obj *unstructured.Unstructured) string {
	id := obj.GetAnnotations()[inventory.OwningInventoryKey]
	return id
}
