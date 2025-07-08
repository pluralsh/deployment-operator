package template

import (
	"fmt"
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
	Render(svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]unstructured.Unstructured, error) {
	if len(svc.Renderers) == 0 {
		return renderDefault(dir, svc, utilFactory)
	}

	var allManifests []unstructured.Unstructured
	for _, renderer := range svc.Renderers {
		var manifests []unstructured.Unstructured
		var err error

		switch renderer.Type {
		case console.RendererTypeAuto:
			manifests, err = renderDefault(renderer.Path, svc, utilFactory)
		case console.RendererTypeRaw:
			manifests, err = NewRaw(renderer.Path).Render(svc, utilFactory)
		case console.RendererTypeHelm:
			manifests, err = NewHelm(renderer.Path).Render(svc, utilFactory)
		case console.RendererTypeKustomize:
			manifests, err = NewKustomize(renderer.Path).Render(svc, utilFactory)
		default:
			return nil, fmt.Errorf("unknown renderer type: %s", renderer.Type)
		}

		if err != nil {
			return nil, fmt.Errorf("error rendering path %s with type %s: %w", renderer.Path, renderer.Type, err)
		}

		allManifests = append(allManifests, manifests...)
	}

	return allManifests, nil
}

func renderDefault(dir string, svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]unstructured.Unstructured, error) {
	renderer := RendererRaw

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("error walking path %s: %s\n", path, err)
			return nil
		}

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
