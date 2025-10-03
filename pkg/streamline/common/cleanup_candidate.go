package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// CleanupCandidate represents resources with a specified deletion policy
// that were already processed and are ready for cleanup.
type CleanupCandidate struct {
	UID       string
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
	ServiceID string
}

func (in *CleanupCandidate) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: in.Group, Version: in.Version, Kind: in.Kind}
}

func (in *CleanupCandidate) ToUnstructured() unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(in.GroupVersionKind())
	u.SetNamespace(in.Namespace)
	u.SetName(in.Name)
	u.SetUID(types.UID(in.UID))
	return u
}
