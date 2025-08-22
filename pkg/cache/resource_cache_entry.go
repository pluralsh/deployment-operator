// Package cache provides caching mechanisms for storing and managing resource states,
// including their SHAs and status information from different stages of deployment.
package cache

import (
	"sync"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cli-utils/pkg/apply/event"

	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

type SHAType string

const (
	ManifestSHA SHAType = "MANIFEST"
	ApplySHA    SHAType = "APPLY"
	ServerSHA   SHAType = "SERVER"
)

// ResourceCacheEntry contains latest SHAs for a single resource from multiple stages
// as well as the last seen status of the resource. It tracks different types of SHAs:
// manifest SHA from the repository, apply SHA post-server-side apply, and server SHA from resource watch.
// This allows for drift detection and state management of Kubernetes resources.
type ResourceCacheEntry struct {
	// uid is a k8s resource UID
	uid string

	// manifestSHA is SHA of the resource manifest from the repository.
	// It is used to detect changes in the manifest that are not yet applied.
	manifestSHA *string

	// transientManifestSha is a temporary SHA of the resource manifest from the repository.
	// It is saved by the filters.CacheFilter and committed after the resource is applied.
	transientManifestSha *string

	// applySHA is SHA of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	applySHA *string

	// serverSHA is SHA from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	serverSHA *string

	// status is a simplified Console structure containing last seen status of cache resource.
	status *console.ComponentAttributes

	// mux is a mutex to protect concurrent access to the cache entry.
	mux sync.Mutex
}

func (in *ResourceCacheEntry) GetStatus() *console.ComponentAttributes {
	in.mux.Lock()
	defer in.mux.Unlock()

	return in.status
}

// GetUID returns the Kubernetes resource UID pointer stored in the cache entry.
// The UID uniquely identifies the resource within the Kubernetes cluster.
func (in *ResourceCacheEntry) GetUID() string {
	return in.uid
}

// SetUID sets the Kubernetes resource UID in the cache entry.
func (in *ResourceCacheEntry) SetUID(uid string) {
	in.mux.Lock()
	defer in.mux.Unlock()

	in.uid = uid
}

// Expire implements [Expirable] interface.
func (in *ResourceCacheEntry) Expire() {
	in.mux.Lock()
	defer in.mux.Unlock()

	in.manifestSHA = nil
	in.transientManifestSha = nil
	in.applySHA = nil
	in.status = nil
}

// SetSHA updates shaType with SHA calculated based on the provided resource.
func (in *ResourceCacheEntry) SetSHA(resource unstructured.Unstructured, shaType SHAType) (changed bool, err error) {
	sha, err := store.HashResource(resource)
	if err != nil {
		return false, err
	}

	in.mux.Lock()
	defer in.mux.Unlock()

	switch shaType {
	case ManifestSHA:
		if in.transientManifestSha != nil && *in.transientManifestSha != sha {
			changed = true
		}

		in.transientManifestSha = &sha
	case ApplySHA:
		if in.applySHA != nil && *in.applySHA != sha {
			changed = true
		}

		in.applySHA = &sha
	case ServerSHA:
		if in.serverSHA != nil && *in.serverSHA != sha {
			changed = true
		}

		in.serverSHA = &sha
	}

	return changed, nil
}

// SetStatus saves the last seen resource [event.StatusEvent] and converts it to a simpler
// [console.ComponentAttributes] structure.
func (in *ResourceCacheEntry) SetStatus(se event.StatusEvent) {
	in.mux.Lock()
	defer in.mux.Unlock()

	in.status = common.StatusEventToComponentAttributes(se, make(map[schema.GroupName]string))
}
