package common

import (
	"strings"

	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SyncPhase string

func (sp SyncPhase) Equals(s string) bool {
	return sp.String() == s
}

func (sp SyncPhase) String() string {
	return string(sp)
}

const (
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
)

// SyncPhases contains all currently supported sync phases.
var SyncPhases = []SyncPhase{
	SyncPhasePreSync,
	SyncPhaseSync,
	SyncPhasePostSync,
	SyncPhaseSyncFail,
	SyncPhaseSkip,
}

// GetDeletePhase returns the phase in which the resource should be deleted.
func GetDeletePhase(u unstructured.Unstructured) SyncPhase {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return SyncPhaseSync
	}

	annotation, ok := annotations[SyncPhaseAnnotation]
	if !ok {
		return getHelmDeleteHook(annotations)
	}

	phases := containers.ToSet[string](strings.Split(strings.ReplaceAll(annotation, " ", ""), ","))
	for _, phase := range SyncPhases {
		if phases.Has(phase.String()) {
			return phase
		}
	}

	return getHelmDeleteHook(annotations)
}

func getHelmDeleteHook(annotations map[string]string) SyncPhase {
	annotation, ok := annotations[HelmHookAnnotation]
	if !ok {
		return SyncPhaseSync
	}

	hooks := containers.ToSet[string](strings.Split(strings.ReplaceAll(annotation, " ", ""), ","))
	if hooks.Has(HelmHookPreInstall) || hooks.Has(HelmHookPreUpgrade) {
		return SyncPhasePreSync
	} else if hooks.Has(HelmHookPostInstall) || hooks.Has(HelmHookPostUpgrade) {
		return SyncPhasePostSync
	} else {
		return SyncPhaseSync
	}
}

// HasPhase checks if the resource belongs to the specified sync phase.
func HasPhase(u unstructured.Unstructured, phase SyncPhase, isUpgrade bool) bool {
	annotations := u.GetAnnotations()
	if annotations == nil {
		return phase.Equals(SyncPhaseSync.String()) // If no annotations are found, then put it in the default phase.
	}

	annotation, ok := annotations[SyncPhaseAnnotation]
	if !ok {
		return hasHelmHook(annotations, phase, isUpgrade) // Fallback to Helm annotation check.
	}

	phases := containers.ToSet[string](strings.Split(strings.ReplaceAll(annotation, " ", ""), ","))
	return phases.Has(phase.String())
}

func hasHelmHook(annotations map[string]string, phase SyncPhase, isUpgrade bool) bool {
	annotation, ok := annotations[HelmHookAnnotation]
	if !ok {
		return phase.Equals(SyncPhaseSync.String()) // If no Helm annotation is found, then put it in the default phase.
	}

	hooks := containers.ToSet[string](strings.Split(strings.ReplaceAll(annotation, " ", ""), ","))
	switch phase {
	case SyncPhasePreSync:
		return (hooks.Has(HelmHookPreInstall) && !isUpgrade) || (hooks.Has(HelmHookPreUpgrade) && isUpgrade)
	case SyncPhasePostSync:
		return (hooks.Has(HelmHookPostInstall) && !isUpgrade) || (hooks.Has(HelmHookPostUpgrade) && isUpgrade)
	default:
		return phase.Equals(SyncPhaseSync.String()) // If no matches are found, then put it in the default phase.
	}
}
