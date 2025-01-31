package template

import (
	"os"
	"path/filepath"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Renderer string

const (
	RendererHelm      Renderer = "helm"
	RendererRaw       Renderer = "raw"
	RendererKustomize Renderer = "kustomize"

	ChartFileName = "Chart.yaml"
)

type Template interface {
	Render(svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]*unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	renderer := RendererRaw

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		for _, file := range []string{ChartFileName, "values.yaml"} {
			if !info.IsDir() && info.Name() == file {
				renderer = RendererHelm
				return nil
			}
		}

		if info.Name() == "kustomization.yaml" {
			renderer = RendererKustomize
		}

		return nil
	})

	if svc.Kustomize != nil || renderer == RendererKustomize {
		return NewKustomize(dir).Render(svc, utilFactory)
	}

	if renderer == RendererHelm {
		return NewHelm(dir).Render(svc, utilFactory)
	}

	return NewRaw(dir).Render(svc, utilFactory)
}
