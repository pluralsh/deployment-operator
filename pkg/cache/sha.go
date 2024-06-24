package cache

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SHAType string

const (
	ManifestSHA SHAType = "MANIFEST"
	ApplySHA    SHAType = "APPLY"
	ServerSHA   SHAType = "SERVER"
)

// SHA contains latest SHAs for a single resource from multiple stages.
type SHA struct {
	// manifestSHA is SHA of the resource manifest from the repository.
	manifestSHA *string

	// applySHA is SHA of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	applySHA *string

	// serverSHA is SHA from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	serverSHA *string

	// health is health status of the object found from a watch.
	health *console.ComponentState
}

func (in *SHA) SetSHA(resource unstructured.Unstructured, shaType SHAType) error {
	sha, err := HashResource(resource)
	if err != nil {
		return err
	}

	switch shaType {
	case ManifestSHA:
		in.manifestSHA = &sha
	case ApplySHA:
		in.applySHA = &sha
	case ServerSHA:
		in.serverSHA = &sha
	}

	return nil
}

func (in *SHA) SetManifestSHA(manifestSHA string) {
	in.manifestSHA = &manifestSHA
}

func (in *SHA) SetHealth(resource *unstructured.Unstructured) {
	in.health = getResourceHealth(resource)
}

// RequiresApply checks if there is any drift
// between applySHA calculated during applying resource and serverSHA from a watch of a resource
// or between last two manifestSHA read from the repository.
// If any drift is detected, then server-side apply should be done.
func (in *SHA) RequiresApply(manifestSHA string) bool {
	return in.serverSHA == nil || in.applySHA == nil || in.manifestSHA == nil ||
		*in.serverSHA != *in.applySHA || manifestSHA != *in.manifestSHA
}

// shaObject is a representation of a resource used to calculate SHA from.
type shaObject struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	DeletionTimestamp string            `json:"deletionTimestamp"`
	Other             map[string]any    `json:"other"`
}

// HashResource calculates SHA for an unstructured object.
// It uses object metadata (name, namespace, labels, annotations, deletion timestamp)
// and all other top-level fields except status.
func HashResource(resource unstructured.Unstructured) (string, error) {
	resourceCopy := resource.DeepCopy()
	object := shaObject{
		Name:        resourceCopy.GetName(),
		Namespace:   resourceCopy.GetNamespace(),
		Labels:      resourceCopy.GetLabels(),
		Annotations: resourceCopy.GetAnnotations(),
	}

	if resourceCopy.GetDeletionTimestamp() != nil {
		object.DeletionTimestamp = resourceCopy.GetDeletionTimestamp().String()
	}

	unstructured.RemoveNestedField(resourceCopy.Object, "metadata")
	unstructured.RemoveNestedField(resourceCopy.Object, "status")
	object.Other = resourceCopy.Object

	return utils.HashObject(object)
}

// getResourceHealth returns the health of a resource.
func getResourceHealth(obj *unstructured.Unstructured) *console.ComponentState {
	if obj.GetDeletionTimestamp() != nil {
		return lo.ToPtr(console.ComponentStatePending)
	}

	healthCheckFunc := common.GetHealthCheckFuncByGroupVersionKind(obj.GroupVersionKind())
	if healthCheckFunc == nil {
		return lo.ToPtr(console.ComponentStatePending)
	}

	health, err := healthCheckFunc(obj)
	if err != nil {
		return nil
	}

	switch health.Status {
	case common.HealthStatusDegraded:
		return lo.ToPtr(console.ComponentStateFailed)
	case common.HealthStatusHealthy:
		return lo.ToPtr(console.ComponentStateRunning)
	case common.HealthStatusPaused:
		return lo.ToPtr(console.ComponentStatePaused)
	}

	return lo.ToPtr(console.ComponentStatePending)
}
