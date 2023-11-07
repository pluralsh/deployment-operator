package template

import (
	"os"
	"path/filepath"

	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Renderer string

const (
	RendererHelm Renderer = "helm"
	RendererRaw  Renderer = "raw"
)

type Template interface {
	Render(svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	renderer := RendererRaw

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		for _, file := range []string{"Chart.yaml", "values.yaml"} {
			if !info.IsDir() && info.Name() == file {
				renderer = RendererHelm
				return nil
			}
		}

		return nil
	})

	if svc.Kustomize != nil {
		return NewKustomize(dir).Render(svc, utilFactory)
	}

	if renderer == RendererHelm {
		return NewHelm(dir).Render(svc, utilFactory)
	}

	return NewRaw(dir).Render(svc, utilFactory)
}
