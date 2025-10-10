package common

import (
	"github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// HookComponent represents hook resources that have deletion policy set.
type HookComponent struct {
	UID         string
	Group       string
	Version     string
	Kind        string
	Name        string
	Namespace   string
	Status      string
	ManifestSHA string
	ServiceID   string
}

func (in *HookComponent) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: in.Group, Version: in.Version, Kind: in.Kind}
}

func (in *HookComponent) Succeeded() bool {
	return in.Status == string(client.ComponentStateRunning)
}

func (in *HookComponent) Failed() bool {
	return in.Status == string(client.ComponentStateFailed)
}

func (in *HookComponent) HasDesiredState(deletionPolicy string) bool {
	switch deletionPolicy {
	case SyncPhaseDeletePolicySucceeded:
		return in.Succeeded()
	case SyncPhaseDeletePolicyFailed:
		return in.Failed()
	default:
		return false
	}
}

func (in *HookComponent) HasManifestChanged(u unstructured.Unstructured) bool {
	sha, _ := utils.HashResource(u)
	return in.ManifestSHA != sha
}

func (in *HookComponent) Unstructured() unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(in.GroupVersionKind())
	u.SetNamespace(in.Namespace)
	u.SetName(in.Name)
	u.SetUID(types.UID(in.UID))
	return u
}

func (in *HookComponent) StoreKey() StoreKey {
	return StoreKey{GVK: in.GroupVersionKind(), Namespace: in.Namespace, Name: in.Name}
}
