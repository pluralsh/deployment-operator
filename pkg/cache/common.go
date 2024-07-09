package cache

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/internal/utils"
)

// shaObject is a helper structure that represents a resource used to calculate SHA.
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
