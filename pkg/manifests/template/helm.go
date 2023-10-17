package template

import (
	"bytes"
	"fmt"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/fs"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
)

type helm struct {
	dir string
}

func (h *helm) Render(svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	// helm's k8s client version conflicts with gitops-engine, need to manually shell out (this is also how argo handles it apparently)
	outb, errb := bytes.Buffer{}, bytes.Buffer{}

	// TODO: add some configured values file convention, perhaps using our lua templating from plural-cli
	args := []string{"template", svc.Name, h.dir, "--namespace", svc.Namespace, "--include-crds"}
	f, err := h.values(svc)
	if err != nil {
		return nil, err
	}
	if f != "" {
		args = append(args, "-f", f)
		defer os.Remove(f)
	}

	cmd := exec.Command("helm", args...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("could not template helm chart: err=%s, out=%s", errb.Bytes(), outb.Bytes())
	}

	r := bytes.NewReader(outb.Bytes())

	mapper, err := utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	readerOptions := manifestreader.ReaderOptions{
		Mapper:           mapper,
		Namespace:        svc.Namespace,
		EnforceNamespace: true,
	}
	mReader := &manifestreader.StreamManifestReader{
		ReaderName:    "helm",
		Reader:        r,
		ReaderOptions: readerOptions,
	}

	items, err := mReader.Read()
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (h *helm) values(svc *console.ServiceDeploymentExtended) (path string, err error) {
	lqPath := filepath.Join(h.dir, "values.yaml.liquid")
	var data []byte
	if fs.Exists(lqPath) {
		data, err = os.ReadFile(lqPath)
		if err != nil {
			return
		}

		data, err = renderLiquid(data, svc)
		if err != nil {
			return
		}

		return fs.TmpFile(fmt.Sprintf("%s-%s.yaml", svc.Namespace, svc.Name), data)
	}

	return
}

func NewHelm(dir string) Template {
	return &helm{dir}
}
