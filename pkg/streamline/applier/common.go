package applier

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func asJSON(resource *unstructured.Unstructured) string {
	data, _ := yaml.Marshal(resource)
	return string(data)
}
