package applier

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

type Key string

func NewKeyFromEntry(entry store.Entry) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", entry.Group, entry.Version, entry.Kind, entry.Namespace, entry.Name))
}

func NewKeyFromUnstructured(unstructured unstructured.Unstructured) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", unstructured.GroupVersionKind().Group, unstructured.GroupVersionKind().Version, unstructured.GroupVersionKind().Kind, unstructured.GetNamespace(), unstructured.GetName()))
}
