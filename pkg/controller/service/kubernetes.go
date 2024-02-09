package service

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

func asJSON(resource *unstructured.Unstructured) string {
	data, _ := yaml.Marshal(resource)
	return string(data)
}

func fetch(client *kubernetes.Clientset, resource *unstructured.Unstructured) *unstructured.Unstructured {
	if client == nil || resource == nil {
		return nil
	}

	response := new(unstructured.Unstructured)
	err := client.RESTClient().Get().AbsPath(toAPIPath(resource)).Do(context.Background()).Into(response)
	if err != nil {
		return nil
	}

	return response
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

