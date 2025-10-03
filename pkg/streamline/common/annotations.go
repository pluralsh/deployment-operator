package common

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/pkg/common"
)

type SyncPhase string

func (s SyncPhase) String() string {
	return string(s)
}

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

	// HelmHookWeightAnnotation is the annotation key used to store the helm hook weight
	HelmHookWeightAnnotation = "helm.sh/hook-weight"

	// HelmHookAnnotation is the annotation key used to store the helm hook type
	// that should be applied during specific phases of the applying lifecycle.
	HelmHookAnnotation = "helm.sh/hook"

	// HelmHookPreInstall resources are applied before the installation of resources.
	HelmHookPreInstall = "pre-install"

	// HelmHookPostInstall resources are applied after the installation of resources.
	HelmHookPostInstall = "post-install"

	// HelmHookPreUpgrade resources are applied before the upgrade of resources.
	HelmHookPreUpgrade = "pre-upgrade"

	// HelmHookPostUpgrade resources are applied after the upgrade of resources.
	HelmHookPostUpgrade = "post-upgrade"

	// SyncWaveAnnotation allows users to customize resource apply ordering when needed.
	SyncWaveAnnotation = "deployment.plural.sh/sync-wave"

	// SyncPhaseAnnotation allows users to customize resource apply phases when needed.
	SyncPhaseAnnotation = "deployment.plural.sh/sync-hook"

	// SyncPhasePreSync is the earliest phase that a resource can be in.
	SyncPhasePreSync SyncPhase = "pre-sync"

	// SyncPhaseSync is the default phase that a resource is in. It is applied after the PreSync phase succeeds.
	SyncPhaseSync SyncPhase = "sync"

	// SyncPhasePostSync is the latest phase that a resource can be in. It is applied after the Sync phase succeeds.
	SyncPhasePostSync SyncPhase = "post-sync"

	// SyncPhaseSyncFail is the phase applied when the Sync phase fails.
	SyncPhaseSyncFail SyncPhase = "sync-fail"

	// SyncPhaseSkip means the resource will be skipped during the sync process.
	SyncPhaseSkip SyncPhase = "skip"

	// SyncPhaseHookDeletePolicy allows users to customize resource deletion behavior.
	SyncPhaseHookDeletePolicy = "deployment.plural.sh/sync-hook-delete-policy"

	// SyncPhaseDeletePolicySucceeded means the resource should be deleted if the hook succeeds.
	SyncPhaseDeletePolicySucceeded = "succeeded"

	// SyncPhaseDeletePolicyFailed means the resource should be deleted if the hook fails.
	SyncPhaseDeletePolicyFailed = "failed"

	// defaultSyncPriority should be after the last priority from kindSyncPriorities.
	defaultSyncPriority = 4
)

var (
	SyncPhases = []SyncPhase{
		SyncPhasePreSync,
		SyncPhaseSync,
		SyncPhasePostSync,
		SyncPhaseSyncFail,
		SyncPhaseSkip,
	}

	kindSyncPriorities = map[string]int{
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
func GetSyncWave(u unstructured.Unstructured) int {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return defaultWave(u.GetKind())
	}

	wave, ok := annotations[SyncWaveAnnotation]
	if !ok {
		return helmWave(annotations, u.GetKind())
	}

	i, err := strconv.Atoi(wave)
	if err != nil {
		return helmWave(annotations, u.GetKind())
	}

	return i
}

// helmWave retrieves the helm hook weight from the resource annotations.
func helmWave(annotations map[string]string, kind string) int {
	wave, ok := annotations[HelmHookWeightAnnotation]
	if !ok {
		return defaultWave(kind)
	}

	i, err := strconv.Atoi(wave)
	if err != nil {
		return defaultWave(kind)
	}

	return i
}

// defaultWave returns default sync wave for a resource based on its kind.
// If the sync wave was not defined for a kind, it returns the last default wave.
func defaultWave(kind string) int {
	i, ok := kindSyncPriorities[kind]
	if !ok {
		return defaultSyncPriority
	}

	return i
}

// GetSyncPhase retrieves the sync phase from the resource annotations.
// If the annotation is not present or invalid, it returns the default sync phase.
func GetSyncPhase(u unstructured.Unstructured) SyncPhase {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return SyncPhaseSync
	}

	phase, ok := annotations[SyncPhaseAnnotation]
	if !ok {
		return SyncPhaseSync
	}

	switch phase {
	case string(SyncPhasePreSync):
		return SyncPhasePreSync
	case string(SyncPhasePostSync):
		return SyncPhasePostSync
	case string(SyncPhaseSyncFail):
		return SyncPhaseSyncFail
	case string(SyncPhaseSkip):
		return SyncPhaseSkip
	case string(SyncPhaseSync):
		fallthrough
	default:
		return defaultPhase(annotations)
	}
}

func defaultPhase(annotations map[string]string) SyncPhase {
	hook, ok := annotations[HelmHookAnnotation]
	if !ok {
		return SyncPhaseSync
	}

	switch hook {
	case HelmHookPreInstall:
		fallthrough
	case HelmHookPreUpgrade:
		return SyncPhasePreSync
	case HelmHookPostInstall:
		fallthrough
	case HelmHookPostUpgrade:
		return SyncPhasePostSync
	default:
		return SyncPhaseSync
	}
}

// GetPhaseHookDeletePolicy retrieves the sync phase hook delete policy from the resource annotations.
func GetPhaseHookDeletePolicy(u unstructured.Unstructured) string {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return ""
	}

	return annotations[SyncPhaseHookDeletePolicy]
}
