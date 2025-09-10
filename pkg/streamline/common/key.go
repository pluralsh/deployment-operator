package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Key string

func NewKeyFromEntry(entry Entry) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", entry.Group, entry.Version, entry.Kind, entry.Namespace, entry.Name))
}

func NewKeyFromUnstructured(unstructured unstructured.Unstructured) Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", unstructured.GroupVersionKind().Group, unstructured.GroupVersionKind().Version, unstructured.GroupVersionKind().Kind, unstructured.GetNamespace(), unstructured.GetName()))
}

// StoreKey is a unique identifier for a resource in the store.
// It is using a compound key consisting of GVK, namespace and name as
// CRD object instances can share the same UID across versions.
type StoreKey struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

func (in StoreKey) Key() Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", in.GVK.Group, in.GVK.Version, in.GVK.Kind, in.Namespace, in.Name))
}

func (in StoreKey) VersionlessKey() Key {
	return Key(fmt.Sprintf("%s/%s/%s/%s", in.GVK.Group, in.GVK.Kind, in.Namespace, in.Name))
}

func NewStoreKeyFromEntry(entry Entry) StoreKey {
	return StoreKey{
		GVK: schema.GroupVersionKind{
			Group:   entry.Group,
			Version: entry.Version,
			Kind:    entry.Kind,
		},
		Namespace: entry.Namespace,
		Name:      entry.Name,
	}
}

func NewStoreKeyFromUnstructured(unstructured unstructured.Unstructured) StoreKey {
	return StoreKey{
		GVK: schema.GroupVersionKind{
			Group:   unstructured.GroupVersionKind().Group,
			Version: unstructured.GroupVersionKind().Version,
			Kind:    unstructured.GroupVersionKind().Kind,
		},
		Namespace: unstructured.GetNamespace(),
		Name:      unstructured.GetName(),
	}
}
