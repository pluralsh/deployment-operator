package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	console "github.com/pluralsh/console-client-go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type raw struct {
	dir string
}

func NewRaw(dir string) *raw {
	return &raw{dir}
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var buffer bytes.Buffer
		tpl, err := template.New("gotpl").Funcs(sprig.TxtFuncMap()).Parse(string(data))
		if err != nil {
			return err
		}

		config := configMap(svc)
		if err := tpl.Execute(&buffer, map[string]interface{}{"Values": config}); err != nil {
			return err
		}

		items, err := kube.SplitYAML(buffer.Bytes())
		if err != nil {
			rpath, _ := filepath.Rel(r.dir, path)
			return fmt.Errorf("failed to parse %s: %v", rpath, err)
		}
		res = append(res, items...)
		return nil
	}); err != nil {
		return nil, err
	}

	return res, nil
}
