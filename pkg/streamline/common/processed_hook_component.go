package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ProcessedHookComponent represents hook resources that were already processed and are ready for cleanup.
type ProcessedHookComponent struct {
	UID       string
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	ServiceID string
}

func (in *ProcessedHookComponent) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: in.Group, Version: in.Version, Kind: in.Kind}
}

func (in *ProcessedHookComponent) ToUnstructured() unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(in.GroupVersionKind())
	u.SetNamespace(in.Namespace)
	u.SetName(in.Name)
	u.SetUID(types.UID(in.UID))
	return u
}

func (in *ProcessedHookComponent) ToStoreKey() StoreKey {
	return StoreKey{GVK: in.GroupVersionKind(), Namespace: in.Namespace, Name: in.Name}
}
