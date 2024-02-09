package service

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func asJSON(resource *unstructured.Unstructured) string {
	data, _ := yaml.Marshal(resource)
	return string(data)
}

func toAPIPath(resource *unstructured.Unstructured) string {
	path := strings.Builder{}

	if resource == nil {
		return path.String()
	}

	gvk := resource.GroupVersionKind()
	name := resource.GetName()
	namespace := resource.GetNamespace()

	if len(gvk.Group) > 0 {
		path.WriteString("/apis/")
		path.WriteString(gvk.Group)
		path.WriteString("/")
	} else {
		path.WriteString("/api/")
	}

	path.WriteString(gvk.Version)

	if len(namespace) > 0 {
		path.WriteString("/namespaces/")
		path.WriteString(namespace)
	}

	path.WriteString("/")
	path.WriteString(toPlural(gvk.Kind))
	path.WriteString(name)

	return path.String()
}

func toPlural(kind string) string {
	return fmt.Sprintf("%ss/", strings.ToLower(kind))
}

