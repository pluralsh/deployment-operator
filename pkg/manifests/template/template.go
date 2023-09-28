package template

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path/filepath"
)

type Renderer string

const (
	RendererHelm Renderer = "helm"
	RendererRaw  Renderer = "raw"
)

type Template interface {
	Render(svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error) {
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

	if renderer == RendererHelm {
		return NewHelm(dir).Render(svc)
	}

	return NewRaw(dir).Render(svc)
}
