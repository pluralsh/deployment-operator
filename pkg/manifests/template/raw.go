package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig/v3"
	"github.com/osteele/liquid"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
)

var (
	extensions     = []string{".json", ".yaml", ".yml", ".yaml.liquid", ".yml.liquid", ".json.liquid"}
	liquidEngine   = liquid.NewEngine()
	sprigFunctions = map[string]string{
		"toJson":        "to_json",
		"fromJson":      "from_json",
		"b64enc":        "b64enc",
		"b64dec":        "b64dec",
		"semverCompare": "semver_compare",
		"sha256sum":     "sha26sum",
		"quote":         "quote",
		"squote":        "squote",
		"indent":        "indent",
		"nindent":       "nindent",
		"replace":       "replace",
	}
)

func init() {
	fncs := sprig.TxtFuncMap()
	for key, name := range sprigFunctions {
		liquidEngine.RegisterFilter(name, fncs[key])
	}
}

type raw struct {
	dir string
}

func NewRaw(dir string) *raw {
	return &raw{dir}
}

func isTemplated(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) bool {
	if svc.Templated != nil {
		return *svc.Templated
	}
	// default true
	return true
}

func renderLiquid(input []byte, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) ([]byte, error) {
	bindings := map[string]interface{}{
		"configuration": configMap(svc),
		"cluster":       clusterConfiguration(svc.Cluster),
		"contexts":      contexts(svc),
	}
	return liquidEngine.ParseAndRender(input, bindings)
}

func (r *raw) Render(svc *console.GetServiceDeploymentForAgent_ServiceDeployment, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	res := make([]*unstructured.Unstructured, 0)
	mapper, err := utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	readerOptions := ReaderOptions{
		Mapper:           mapper,
		Namespace:        svc.Namespace,
		EnforceNamespace: false,
	}

	if err := filepath.Walk(r.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(info.Name())); !lo.Contains(extensions, ext) {
			return nil
		}
		rpath, err := filepath.Rel(r.dir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isTemplated(svc) {
			data, err = renderLiquid(data, svc)
			if err != nil {
				return fmt.Errorf("templating error in %s: %w", rpath, err)
			}
		}

		r := bytes.NewReader(data)

		mReader := &StreamManifestReader{
			ReaderName:    "raw",
			Reader:        r,
			ReaderOptions: readerOptions,
		}
		items, err := mReader.Read(res)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", rpath, err)
		}

		res = append(res, items...)
		return nil
	}); err != nil {
		return nil, err
	}
	newSet := containers.ToSet[*unstructured.Unstructured](res)
	res = newSet.List()
	return res, nil
}

func clusterConfiguration(cluster *console.GetServiceDeploymentForAgent_ServiceDeployment_Cluster) map[string]interface{} {
	res := map[string]interface{}{
		"ID":             cluster.ID,
		"Self":           cluster.Self,
		"Handle":         cluster.Handle,
		"Name":           cluster.Name,
		"Version":        cluster.Version,
		"CurrentVersion": cluster.CurrentVersion,
		"KasUrl":         cluster.KasURL,
	}

	for k, v := range res {
		res[strings.ToLower(k)] = v
	}
	res["kasUrl"] = cluster.KasURL
	res["currentVersion"] = cluster.CurrentVersion

	return res
}
