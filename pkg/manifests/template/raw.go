package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/sprig/v3"
	"github.com/osteele/liquid"
	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
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

func renderLiquid(input []byte, svc *console.ServiceDeploymentExtended) ([]byte, error) {
	bindings := map[string]interface{}{
		"configuration": configMap(svc),
		"cluster":       svc.Cluster,
	}
	return liquidEngine.ParseAndRender(input, bindings)
}

func (r *raw) Render(svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	res := make([]*unstructured.Unstructured, 0)
	mapper, err := utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	readerOptions := ReaderOptions{
		Mapper:           mapper,
		Namespace:        svc.Namespace,
		EnforceNamespace: true,
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

		rendered, err := renderLiquid(data, svc)
		if err != nil {
			return fmt.Errorf("templating error in %s: %w", rpath, err)
		}

		r := bytes.NewReader(rendered)

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

	return res, nil
}

// ReaderOptions defines the shared inputs for the different
// implementations of the ManifestReader interface.
type ReaderOptions struct {
	Mapper           meta.RESTMapper
	Validate         bool
	Namespace        string
	EnforceNamespace bool
}

// StreamManifestReader reads manifest from the provided io.Reader
// and returns them as Info objects. The returned Infos will not have
// client or mapping set.
type StreamManifestReader struct {
	ReaderName string
	Reader     io.Reader

	ReaderOptions
}

// Read reads the manifests and returns them as Info objects.
func (r *StreamManifestReader) Read(objs []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	nodes, err := (&kio.ByteReader{
		Reader: r.Reader,
	}).Read()
	if err != nil {
		return objs, err
	}

	for _, n := range nodes {
		err = manifestreader.RemoveAnnotations(n, kioutil.IndexAnnotation)
		if err != nil {
			return objs, err
		}
		u, err := manifestreader.KyamlNodeToUnstructured(n)
		if err != nil {
			return objs, err
		}
		objs = append(objs, u)
	}

	objs = manifestreader.FilterLocalConfig(objs)

	err = manifestreader.SetNamespaces(r.Mapper, objs, r.Namespace, r.EnforceNamespace)
	return objs, err
}
