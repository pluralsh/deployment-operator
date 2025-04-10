package cache

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply/event"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type SHAType string

const (
	ManifestSHA SHAType = "MANIFEST"
	ApplySHA    SHAType = "APPLY"
	ServerSHA   SHAType = "SERVER"
)

// ResourceCacheEntry contains latest SHAs for a single resource from multiple stages
// as well as the last seen status of the resource.
type ResourceCacheEntry struct {
	// manifestSHA is SHA of the resource manifest from the repository.
	manifestSHA *string

	// applySHA is SHA of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	applySHA *string

	// serverSHA is SHA from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	serverSHA *string

	// status is a simplified Console structure containing last seen status of cache resource.
	status *console.ComponentAttributes
}

// Expire implements [Expirable] interface.
func (in *ResourceCacheEntry) Expire() {
	in.manifestSHA = nil
	in.applySHA = nil
	in.status = nil
}

// SetSHA updates shaType with SHA calculated based on the provided resource.
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

// SetManifestSHA updates manifest SHA.
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

// SetStatus saves the last seen resource [event.StatusEvent] and converts it to a simpler
// [console.ComponentAttributes] structure.
func (in *ResourceCacheEntry) SetStatus(ctx context.Context, k8sClient ctrclient.Client, se event.StatusEvent) {
	in.status = common.StatusEventToComponentAttributes(ctx, k8sClient, se, make(map[schema.GroupName]string))
}
