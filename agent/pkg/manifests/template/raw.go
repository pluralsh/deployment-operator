package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig/v3"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/osteele/liquid"
	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	liquidEngine = liquid.NewEngine()
)

func init() {
	for name, fn := range sprig.TxtFuncMap() {
		liquidEngine.RegisterFilter(name, fn)
	}
}

type raw struct {
	dir string
}

func NewRaw(dir string) *raw {
	return &raw{dir}
}

func renderLiquid(input []byte, svc *console.ServiceDeploymentExtended) ([]byte, error) {
	bindings := map[string]interface{}{
		"configuration": configMap(svc),
		"cluster":       svc.Cluster,
	}
	return liquidEngine.ParseAndRender(input, bindings)
}

func (r *raw) Render(svc *console.ServiceDeploymentExtended) ([]*unstructured.Unstructured, error) {
	res := make([]*unstructured.Unstructured, 0)
	if err := filepath.Walk(r.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(info.Name())); ext != ".json" && ext != ".yml" && ext != ".yaml" {
			return nil
		}
		rpath, _ := filepath.Rel(r.dir, path)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rendered, err := renderLiquid(data, svc)
		if err != nil {
			return fmt.Errorf("templating error in %s: %v", rpath, err)
		}

		items, err := kube.SplitYAML(rendered)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %v", rpath, err)
		}
		res = append(res, items...)
		return nil
	}); err != nil {
		return nil, err
	}

	return res, nil
}
