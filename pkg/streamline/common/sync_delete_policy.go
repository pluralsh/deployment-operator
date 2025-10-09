package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// SyncPhaseHookDeletePolicy allows users to customize resource deletion behavior.
	SyncPhaseHookDeletePolicy = "deployment.plural.sh/sync-hook-delete-policy"

	// SyncPhaseDeletePolicySucceeded means the resource should be deleted if the hook succeeds.
	SyncPhaseDeletePolicySucceeded = "succeeded"

	// SyncPhaseDeletePolicyFailed means the resource should be deleted if the hook fails.
	SyncPhaseDeletePolicyFailed = "failed"

	HelmHookDeletePolicyAnnotation = "helm.sh/hook-delete-policy"

	HelmHookDeletePolicyHookSucceeded = "hook-succeeded"

	HelmHookDeletePolicyHookFailed = "hook-failed"
)

// GetPhaseHookDeletePolicy retrieves the sync phase hook delete policy from the resource annotations.
func GetPhaseHookDeletePolicy(u unstructured.Unstructured) string {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return ""
	}

	policy, ok := annotations[SyncPhaseHookDeletePolicy]
	if !ok {
		return helmHookDeletePolicy(annotations)
	}

	switch policy {
	case SyncPhaseDeletePolicySucceeded:
		return SyncPhaseDeletePolicySucceeded
	case SyncPhaseDeletePolicyFailed:
		return SyncPhaseDeletePolicyFailed
	default:
		return helmHookDeletePolicy(annotations)
	}
}

func helmHookDeletePolicy(annotations map[string]string) string {
	policy, ok := annotations[HelmHookDeletePolicyAnnotation]
	if !ok {
		return ""
	}

	switch policy {
	case HelmHookDeletePolicyHookSucceeded:
		return SyncPhaseDeletePolicySucceeded
	case HelmHookDeletePolicyHookFailed:
		return SyncPhaseDeletePolicyFailed
	default:
		return ""
	}
}
