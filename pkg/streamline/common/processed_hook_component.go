package common

import (
	"github.com/pluralsh/console/go/client"
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
	Status    string
	ServiceID string
}

func (in *ProcessedHookComponent) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: in.Group, Version: in.Version, Kind: in.Kind}
}

func (in *ProcessedHookComponent) Succeeded() bool {
	return in.Status == string(client.ComponentStateRunning)
}

func (in *ProcessedHookComponent) Failed() bool {
	return in.Status == string(client.ComponentStateFailed)
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
