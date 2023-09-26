package template

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Template interface {
	Render(svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error) {
	return NewRaw(dir).Render(svc)
}
