package cache

import (
	"github.com/pluralsh/deployment-operator/internal/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	health *string
}

// shaObject is a representation of an object used to calculate SHA from.
type shaObject struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Other       map[string]interface{} `json:"other"`
}

// sha calculates SHA for a unstructured object.
// It uses object name, namespace, labels, annotations and all other top-level fields except status.
func sha(resource unstructured.Unstructured) (string, error) {
	object := shaObject{
		Name:        resource.GetName(),
		Namespace:   resource.GetNamespace(),
		Labels:      resource.GetLabels(),
		Annotations: resource.GetAnnotations(),
	}

	unstructured.RemoveNestedField(resource.Object, "metadata")
	unstructured.RemoveNestedField(resource.Object, "status")
	object.Other = resource.Object

	return utils.HashObject(object)
}
