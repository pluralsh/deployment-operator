package manifests

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
)

func VersionCache(manifests []unstructured.Unstructured) map[schema.GroupName]string {
	res := map[schema.GroupName]string{}
	for _, man := range manifests {
		gvk := man.GroupVersionKind()
		name := schema.GroupName{
			Group: gvk.Group,
			Kind:  gvk.Kind,
			Name:  man.GetName(),
		}
		res[name] = gvk.Version
	}
	return res
}
