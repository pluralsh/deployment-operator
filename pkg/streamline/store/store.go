package store

import (
	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type Entry struct {
	UID                  string
	ParentUID            string
	Group                string
	Version              string
	Kind                 string
	Name                 string
	Namespace            string
	Status               string
	ServiceID            string
	ManifestSHA          string
	TransientManifestSHA string
	ApplySHA             string
	ServerSHA            string
}

// ShouldApply determines if a resource should be applied based on its SHAs.
// Resource should be applied if:
// - any of the SHAs (Server, Apply, or Manifest) are not set
// - the current server SHA differs from stored apply SHA (indicating resource changed in cluster)
// - the new manifest SHA differs from stored manifest SHA (indicating the manifest has changed)
func (in *Entry) ShouldApply(newManifestSHA string) bool {
	return in.ServerSHA == "" || in.ApplySHA == "" || in.ManifestSHA == "" ||
		in.ServerSHA != in.ApplySHA || newManifestSHA != in.ManifestSHA
}

func (in *Entry) ToComponentAttributes() client.ComponentAttributes {
	return client.ComponentAttributes{
		UID:       lo.ToPtr(in.UID),
		Synced:    true,
		Group:     in.Group,
		Version:   in.Version,
		Kind:      in.Kind,
		Name:      in.Name,
		Namespace: in.Namespace,
		State:     lo.ToPtr(client.ComponentState(in.Status)),
	}
}

type SHAType string

const (
	// ManifestSHA is SHA of the resource manifest from the repository.
	// It is used to detect changes in the manifest that are not yet applied.
	ManifestSHA SHAType = "MANIFEST"

	// ApplySHA is SHA of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	ApplySHA SHAType = "APPLY"

	// ServerSHA is SHA from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	ServerSHA SHAType = "SERVER"

	// TransientManifestSHA is a temporary SHA of the resource manifest from the repository.
	// It is saved by the filters and committed after the resource is applied.
	TransientManifestSHA SHAType = "TRANSIENT"
)

type Store interface {
	SaveComponent(obj unstructured.Unstructured) error

	SaveComponentAttributes(obj client.ComponentChildAttributes, args ...any) error

	GetComponent(obj unstructured.Unstructured) (*Entry, error)

	GetComponentByUID(uid types.UID) (*client.ComponentChildAttributes, error)

	// DeleteComponent removes a component from the store based on its UID.
	// It returns an error if any issue occurs during the deletion process.
	DeleteComponent(uid types.UID) error

	// GetServiceComponents retrieves all components associated with a given service ID.
	// It returns a slice of Entry structs containing information about each component and any error encountered.
	GetServiceComponents(serviceID string) ([]Entry, error)

	// GetComponentChildren retrieves all child components and their descendants up to 4 levels deep for a given component UID.
	// It returns a slice of ComponentChildAttributes containing information about each child component and any error encountered.
	GetComponentChildren(uid string) ([]client.ComponentChildAttributes, error)

	GetComponentInsights() ([]client.ClusterInsightComponentAttributes, error)

	GetComponentCounts() (nodeCount, namespaceCount int64, err error)

	// GetNodeStatistics returns a list of node statistics, including the node name and count of pending pods
	// that were created more than 5 minutes ago. Each NodeStatisticAttributes contains the node name and
	// the number of pending pods for that node. The health field is currently not implemented.
	// Returns an error if the database operation fails or if the connection cannot be established.
	GetNodeStatistics() ([]*client.NodeStatisticAttributes, error)

	// GetHealthScore calculates cluster health as a percentage using a base score minus deductions system.
	// Returns the health score as an integer value (0-100) and any error encountered.
	GetHealthScore() (int64, error)

	UpdateComponentSHA(unstructured.Unstructured, SHAType) error

	CommitTransientSHA(unstructured.Unstructured) error

	ExpireSHA(unstructured.Unstructured) error

	Shutdown() error
}
