package watch

import (
	"sync"

	"github.com/pluralsh/polly/containers"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var DefaultK8sResources = containers.ToSet([]schema.GroupVersionResource{
	// Core API group (v1)
	{Group: "", Version: "v1", Resource: "pods"},
	{Group: "", Version: "v1", Resource: "services"},
	{Group: "", Version: "v1", Resource: "endpoints"},
	{Group: "", Version: "v1", Resource: "namespaces"},
	{Group: "", Version: "v1", Resource: "nodes"},
	{Group: "", Version: "v1", Resource: "persistentvolumes"},
	{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	{Group: "", Version: "v1", Resource: "resourcequotas"},
	{Group: "", Version: "v1", Resource: "secrets"},
	{Group: "", Version: "v1", Resource: "configmaps"},
	{Group: "", Version: "v1", Resource: "events"},

	// Apps API group
	{Group: "apps", Version: "v1", Resource: "deployments"},
	{Group: "apps", Version: "v1", Resource: "replicasets"},
	{Group: "apps", Version: "v1", Resource: "statefulsets"},
	{Group: "apps", Version: "v1", Resource: "daemonsets"},

	// Batch API group
	{Group: "batch", Version: "v1", Resource: "jobs"},
	{Group: "batch", Version: "v1", Resource: "cronjobs"},

	// Networking API group
	{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},

	// RBAC API group
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
})

var resources *Resources

func init() {
	resources = &Resources{
		resourceKeySet: DefaultK8sResources,
	}
}

func GetResources() *Resources {
	return resources
}

type Resources struct {
	mu sync.Mutex

	resourceKeySet containers.Set[schema.GroupVersionResource]
}

func (in *Resources) Register(inventoryResourceKeys containers.Set[schema.GroupVersionResource]) {
	in.mu.Lock()
	defer in.mu.Unlock()

	inventoryResourceKeys = inventoryResourceKeys.Union(DefaultK8sResources)
	toAdd := inventoryResourceKeys.Difference(in.resourceKeySet)

	if len(toAdd) > 0 {
		in.resourceKeySet = containers.ToSet(append(in.resourceKeySet.List(), inventoryResourceKeys.List()...))
	}
}

func (in *Resources) UnregisterResource(crd *apiextensionsv1.CustomResourceDefinition) {
	in.mu.Lock()
	defer in.mu.Unlock()

	keys := containers.NewSet[schema.GroupVersionResource]()
	for _, v := range crd.Spec.Versions {
		if v.Served { // only use served versions
			keys.Add(schema.GroupVersionResource{
				Group:    crd.Spec.Group,
				Version:  v.Name,
				Resource: crd.Spec.Names.Plural,
			})
		}
	}

	in.resourceKeySet = keys.Difference(in.resourceKeySet)
}

func (in *Resources) GetResourceKeySet() containers.Set[schema.GroupVersionResource] {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.resourceKeySet
}
