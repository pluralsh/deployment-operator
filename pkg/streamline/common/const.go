package common

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// LifecycleDeleteAnnotation is the lifecycle annotation key for a deletion operation.
	// Keep it the same as cli-utils for backwards compatibility.
	LifecycleDeleteAnnotation = "client.lifecycle.config.k8s.io/deletion"

	// PreventDeletion is the value used with LifecycleDeletionAnnotation
	// to prevent deleting a resource. Keep it the same as cli-utils
	// for backwards compatibility.
	PreventDeletion = "detach"

	// ClientFieldManager is a name associated with the actor or entity
	// that is making changes to the object. Keep it the same as cli-utils
	// for backwards compatibility.
	ClientFieldManager = "application/apply-patch"

	// OwningInventoryKey is the key used to store the owning service id
	// in the annotations of a resource.
	OwningInventoryKey = "config.k8s.io/owning-inventory"

	// TrackingIdentifierKey is the key used to store the unique identifier
	// of a resource in the annotations of a resource.
	// This is used to make sure that the owning inventory was not copied from another resource.
	TrackingIdentifierKey = "config.k8s.io/tracking-identifier"

	// SyncWaveAnnotation allows users to customize resource apply ordering when needed.
	SyncWaveAnnotation = "deployment.plural.sh/sync-wave"
)

func GetOwningInventory(obj unstructured.Unstructured) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}

	serviceID := annotations[OwningInventoryKey]
	if serviceID == "" || !ValidateTrackingIdentifier(obj) {
		return ""
	}

	return serviceID
}

func GetTrackingIdentifier(obj unstructured.Unstructured) string {
	if annotations := obj.GetAnnotations(); annotations != nil {
		return annotations[TrackingIdentifierKey]
	}

	return ""
}

// ValidateTrackingIdentifier checks if the key created from the resource metadata
// is equal to the key in the tracking identifier annotation.
// If that is not the case, then it is likely that the annotation was copied
// from another resource, and we should not trust it and the owning inventory annotation.
func ValidateTrackingIdentifier(u unstructured.Unstructured) bool {
	return NewKeyFromUnstructured(u).Equals(GetTrackingIdentifier(u))
}

// GetSyncWave retrieves the sync wave from the resource annotations.
// If the annotation is not present or invalid, it returns nil.
func GetSyncWave(obj unstructured.Unstructured) *int {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	wave, ok := annotations[SyncWaveAnnotation]
	if !ok {
		return nil
	}

	waveInt, err := strconv.Atoi(wave)
	if err != nil {
		return nil
	}

	return &waveInt
}
