package common

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

const (
	// SyncOptionsAnnotation specifies sync options for a given resource.
	SyncOptionsAnnotation = "deployment.plural.sh/sync-options"

	// ArgoSyncOptionsAnnotation specifies sync options for a given resource.
	ArgoSyncOptionsAnnotation = "argocd.argoproj.io/sync-options"

	// SyncOptionForce indicates if a resource should be forcefully applied during sync.
	// If the initial applying fails, then the resource will be deleted and recreated forcefully.
	SyncOptionForce = "Force=true"
)

func HasSyncOption(u unstructured.Unstructured, option string) bool {
	return true // TODO
}

func HasForceSyncOption(u unstructured.Unstructured) bool {
	return HasSyncOption(u, SyncOptionForce)
}
