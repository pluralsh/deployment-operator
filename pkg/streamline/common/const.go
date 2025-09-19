package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// LifecycleDeleteAnnotation is the lifecycle annotation key for deletion operation.
	// Keep it the same as cli-utils for backwards compatibility.
	LifecycleDeleteAnnotation = "client.lifecycle.config.k8s.io/deletion"

	// PreventDeletion is the value used with LifecycleDeletionAnnotation
	// to prevent deleting a resource. Keep it the same as cli-utils
	// for backwards compatibility.
	PreventDeletion = "detach"

	// OwningInventoryKey is the key used to store the owning service id
	// in the annotations of a resource.
	OwningInventoryKey = "config.k8s.io/owning-inventory"

	// ClientFieldManager is a name associated with the actor or entity
	// that is making changes to the object. Keep it the same as cli-utils
	// for backwards compatibility.
	ClientFieldManager = "application/apply-patch"
)

func GetOwningInventory(obj unstructured.Unstructured) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		return annotations[OwningInventoryKey]
	}

	return ""
}
