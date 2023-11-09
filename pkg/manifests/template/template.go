package template

import (
	"github.com/pluralsh/deployment-operator/pkg/hook"
	"os"
	"path/filepath"

	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Renderer string

const (
	RendererHelm    Renderer = "helm"
	RendererRaw     Renderer = "raw"
	RenderKustomize Renderer = "kustomize"
ChartFileName = "Chart.yaml"
)

type Template interface {
	Render(svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error)
}

func Render(dir string, svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	renderer := RendererRaw

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		for _, file := range []string{ChartFileName, "values.yaml"} {
			if !info.IsDir() && info.Name() == file {
				renderer = RendererHelm
				return nil
			}
		}

		return nil
	})

	if svc.Kustomize != nil {
		renderer = RenderKustomize
	}
	var targets []*unstructured.Unstructured
	var hooks []*unstructured.Unstructured
	var err error

	switch renderer {
	case RenderKustomize:
		targets, err = NewKustomize(dir).Render(svc, utilFactory)
	case RendererHelm:
		targets, err = NewHelm(dir).Render(svc, utilFactory)
		if err != nil {
			return nil, nil, err
		}
		targets, hooks = hook.SplitHooks(targets)
	default:
		targets, err = NewRaw(dir).Render(svc, utilFactory)
	}

	return targets, hooks, err
}
