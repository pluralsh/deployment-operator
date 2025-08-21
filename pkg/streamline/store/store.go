package store

import (
	"github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type Entry struct {
	UID       string
	ParentUID string
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	Status    string
	ServiceID string

	// ManifestSHA is SHA of the resource manifest from the repository.
	// It is used to detect changes in the manifest that are not yet applied.
	ManifestSHA string

	// TransientManifestSHA is a temporary SHA of the resource manifest from the repository.
	// It is saved by the filters and committed after the resource is applied.
	TransientManifestSHA string

	// ApplySHA is SHA of the resource post-server-side apply.
	// Taking only metadata w/ name, namespace, annotations and labels and non-status non-metadata fields.
	ApplySHA string

	// ServerSHA is SHA from a watch of the resource, using the same pruning function as applySHA.
	// It is persisted only if there's a current-inventory annotation.
	ServerSHA string
}

type SHAType string

const (
	ManifestSHA  SHAType = "MANIFEST"
	ApplySHA     SHAType = "APPLY"
	ServerSHA    SHAType = "SERVER"
	TransientSHA SHAType = "TRANSIENT"
)

type Store interface {
	SaveComponent(obj unstructured.Unstructured) error

	SaveComponentAttributes(obj client.ComponentChildAttributes, args ...any) error

	GetComponent(obj unstructured.Unstructured) (result *Entry, err error)

	GetComponentByUID(uid string) (result *client.ComponentChildAttributes, err error)

	DeleteComponent(uid types.UID) error

	GetServiceComponents(serviceID string) ([]Entry, error)

	// GetComponentChildren retrieves all child components and their descendants up to 4 levels deep for a given component UID.
	// It returns a slice of ComponentChildAttributes containing information about each child component.
	//
	// Parameters:
	//   - uid: The unique identifier of the parent component
	//
	// Returns:
	//   - []ComponentChildAttributes: A slice containing the child components and their attributes
	//   - error: An error if the database operation fails or if the connection cannot be established
	GetComponentChildren(uid string) (result []client.ComponentChildAttributes, err error)

	GetComponentInsights() (result []client.ClusterInsightComponentAttributes, err error)

	GetComponentCounts() (nodeCount, namespaceCount int64, err error)

	// GetNodeStatistics returns a list of node statistics, including the node name and count of pending pods
	// that were created more than 5 minutes ago. Each NodeStatisticAttributes contains the node name and
	// the number of pending pods for that node. The health field is currently not implemented.
	// Returns an error if the database operation fails or if the connection cannot be established.
	GetNodeStatistics() ([]*client.NodeStatisticAttributes, error)

	GetHealthScore() (int64, error)

	Shutdown() error
}
