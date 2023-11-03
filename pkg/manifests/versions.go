package manifests

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type GroupName struct {
	Group string
	Kind  string
	Name  string
}

func VersionCache(manifests []*unstructured.Unstructured) map[GroupName]string {
	res := map[GroupName]string{}
	for _, man := range manifests {
		gvk := man.GroupVersionKind()
		name := GroupName{
			Group: gvk.Group,
			Kind:  gvk.Kind,
			Name:  man.GetName(),
		}
		res[name] = gvk.Version
	}
	return res
}
