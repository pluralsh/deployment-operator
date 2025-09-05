package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Key string

func NewKeyFromEntry(entry Entry) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", entry.Group, entry.Version, entry.Kind, entry.Namespace, entry.Name))
}

func NewKeyFromUnstructured(unstructured unstructured.Unstructured) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", unstructured.GroupVersionKind().Group, unstructured.GroupVersionKind().Version, unstructured.GroupVersionKind().Kind, unstructured.GetNamespace(), unstructured.GetName()))
}
