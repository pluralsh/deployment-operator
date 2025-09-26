package common

import (
	"strconv"

	"github.com/pluralsh/deployment-operator/pkg/common"
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

var (
	kindToDefaultSyncWave = map[string]int{
		// Wave 0 - core non-namespaced resources
		common.NamespaceKind:                0,
		common.CustomResourceDefinitionKind: 0,
		common.PersistentVolumeKind:         0,
		common.ClusterRoleKind:              0,
		common.ClusterRoleListKind:          0,
		common.ClusterRoleBindingKind:       0,
		common.ClusterRoleBindingListKind:   0,
		common.StorageClassKind:             0,

		// Wave 1 - core namespaced configuration resources
		common.ConfigMapKind:             1,
		common.SecretKind:                1,
		common.SecretListKind:            1,
		common.ServiceAccountKind:        1,
		common.RoleKind:                  1,
		common.RoleListKind:              1,
		common.RoleBindingKind:           1,
		common.RoleBindingListKind:       1,
		common.PodDisruptionBudgetKind:   1,
		common.ResourceQuotaKind:         1,
		common.NetworkPolicyKind:         1,
		common.LimitRangeKind:            1,
		common.PodSecurityPolicyKind:     1,
		common.IngressClassKind:          1,
		common.PersistentVolumeClaimKind: 1,

		// Wave 2 - core namespaced workload resources
		common.DeploymentKind:            2,
		common.DaemonSetKind:             2,
		common.StatefulSetKind:           2,
		common.ReplicaSetKind:            2,
		common.JobKind:                   2,
		common.CronJobKind:               2,
		common.PodKind:                   2,
		common.ReplicationControllerKind: 2,

		// Wave 3 - core namespaced networking resources
		common.EndpointsKind:  3,
		common.ServiceKind:    3,
		common.IngressKind:    3,
		common.APIServiceKind: 3,
	}

	lastDefaultWave = 3 // TODO
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
func GetSyncWave(u unstructured.Unstructured) int {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return defaultWave(u)
	}

	wave, ok := annotations[SyncWaveAnnotation]
	if !ok {
		return defaultWave(u)
	}

	i, err := strconv.Atoi(wave)
	if err != nil {
		return defaultWave(u)
	}

	return i
}

// defaultWave returns default sync wave for a resource based on its kind.
// If the sync wave was not defined for a kind, it returns the last default wave.
func defaultWave(u unstructured.Unstructured) int {
	i, ok := kindToDefaultSyncWave[u.GetKind()]
	if !ok {
		return lastDefaultWave
	}

	return i
}
