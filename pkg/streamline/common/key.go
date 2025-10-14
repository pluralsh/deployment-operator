package common

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Key string

func (k Key) Equals(s string) bool {
	return string(k) == s
}

func (k Key) String() string {
	return string(k)
}

func NewKeyFromUnstructured(u unstructured.Unstructured) Key {
	gvk := u.GroupVersionKind()
	return Key(fmt.Sprintf("%s/%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, u.GetNamespace(), u.GetName()))
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

func (in StoreKey) ReplaceGroup(group string) StoreKey {
	return StoreKey{
		GVK: schema.GroupVersionKind{
			Group:   group,
			Version: in.GVK.Version,
			Kind:    in.GVK.Kind,
		},
		Namespace: in.Namespace,
		Name:      in.Name,
	}
}

func NewStoreKeyFromUnstructured(u unstructured.Unstructured) StoreKey {
	return StoreKey{GVK: u.GroupVersionKind(), Namespace: u.GetNamespace(), Name: u.GetName()}
}
