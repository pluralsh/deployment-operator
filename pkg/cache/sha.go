package cache

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type SHAType string

const (
	ManifestSHA SHAType = "MANIFEST"
	ApplySHA    SHAType = "APPLY"
	ServerSHA   SHAType = "SERVER"
)

// ResourceCacheEntry contains latest SHAs for a single resource from multiple stages.
type ResourceCacheEntry struct {
	// manifestSHA is ResourceCacheEntry of the resource manifest from the repository.
	manifestSHA *string

	// applySHA is ResourceCacheEntry of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	applySHA *string

	// serverSHA is ResourceCacheEntry from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	serverSHA *string

	status *console.ComponentAttributes
}

// Expire implements [Expirable] interface.
func (in *ResourceCacheEntry) Expire() {
	in.manifestSHA = nil
	in.applySHA = nil
}

func (in *ResourceCacheEntry) SetSHA(resource unstructured.Unstructured, shaType SHAType) error {
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

func (in *ResourceCacheEntry) SetManifestSHA(manifestSHA string) {
	in.manifestSHA = &manifestSHA
}

// RequiresApply checks if there is any drift
// between applySHA calculated during applying resource and serverSHA from a watch of a resource
// or between last two manifestSHA read from the repository.
// If any drift is detected, then server-side apply should be done.
func (in *ResourceCacheEntry) RequiresApply(manifestSHA string) bool {
	return in.serverSHA == nil ||
		in.applySHA == nil ||
		in.manifestSHA == nil ||
		(*in.serverSHA != *in.applySHA) ||
		(manifestSHA != *in.manifestSHA)
}

// shaObject is a representation of a resource used to calculate ResourceCacheEntry from.
type shaObject struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	DeletionTimestamp string            `json:"deletionTimestamp"`
	Other             map[string]any    `json:"other"`
}

// HashResource calculates ResourceCacheEntry for an unstructured object.
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

func (in *ResourceCacheEntry) SetStatus(status *console.ComponentAttributes) {
	in.status = status
}
