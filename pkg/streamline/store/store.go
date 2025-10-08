package store

import (
	"time"

	"github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
)

type Store interface {
	SaveComponent(obj unstructured.Unstructured) error

	SaveComponents(obj []unstructured.Unstructured) error

	SaveComponentAttributes(obj client.ComponentChildAttributes, args ...any) error

	GetComponent(obj unstructured.Unstructured) (*smcommon.Component, error)

	GetComponentByUID(uid types.UID) (*client.ComponentChildAttributes, error)

	GetComponentsByGVK(gvk schema.GroupVersionKind) ([]smcommon.Component, error)

	// DeleteComponent removes a component from the store based on its smcommon.StoreKey.
	// It returns an error if any issue occurs during the deletion process.
	DeleteComponent(key smcommon.StoreKey) error

	// DeleteComponents removes components from the store based on GVK.
	// It returns an error if any issue occurs during the deletion process.
	DeleteComponents(group, version, kind string) error

	// GetServiceComponents retrieves all parent components associated with a given service ID.
	// All components with parents are filtered out.
	// It returns a slice of Component structs containing information about each component and any error encountered.
	GetServiceComponents(serviceID string) ([]smcommon.Component, error)

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

	// UpdateComponentSHA updates the SHA for a component.
	UpdateComponentSHA(unstructured.Unstructured, SHAType) error

	// CommitTransientSHA commits a transient SHA to the store.
	CommitTransientSHA(unstructured.Unstructured) error

	// ExpireSHA removes component SHA information.
	ExpireSHA(unstructured.Unstructured) error

	// Expire removes component SHA information based on the provided service ID.
	Expire(string) error

	// ExpireOlderThan removes component SHA information from entries older than the provided TTL.
	ExpireOlderThan(time.Duration) error

	// Shutdown closes the database connection and deletes the store.
	Shutdown() error

	// GetResourceHealth checks health statuses of provided resources.
	GetResourceHealth(resources []unstructured.Unstructured) (pending, failed bool, err error)

	// HasSomeResources checks if at least one of the provided resources exists in the store.
	HasSomeResources(resources []unstructured.Unstructured) (bool, error)

	// GetHookComponents returns all hook components with a deletion policy that belong to the specified service.
	GetHookComponents(serviceID string) ([]smcommon.HookComponent, error)

	// SaveHookComponentWithManifestSHA saves manifest SHA for the given hook component.
	SaveHookComponentWithManifestSHA(manifest, appliedResource unstructured.Unstructured) error

	// ExpireHookComponents removes all hook components that belong to the specified service from the store.
	ExpireHookComponents(serviceID string) error
}
