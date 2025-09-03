package applier

const (
	// LifecycleDeletionAnnotation is the lifecycle annotation key for deletion operation.
	LifecycleDeleteAnnotation = "plural.sh/deletion"

	// PreventDeletion is the value used with LifecycleDeletionAnnotation
	// to prevent deleting a resource.
	PreventDeletion = "detach"

	OwningInventoryKey = "config.k8s.io/owning-inventory"
)
