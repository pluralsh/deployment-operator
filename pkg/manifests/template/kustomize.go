package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/kustomize/kustomize/v5/commands/build"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type kustomize struct {
	dir string
}

func NewKustomize(dir string) Template {
	return &kustomize{dir}
}

func (k *kustomize) Render(svc *console.GetServiceDeploymentForAgent_ServiceDeployment, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	out := &bytes.Buffer{}
	h := build.MakeHelp("plural", "kustomize")
	help := &build.Help{
		Use:     h.Use,
		Short:   i18n.T(h.Short),
		Long:    templates.LongDesc(i18n.T(h.Long)),
		Example: templates.Examples(i18n.T(h.Example)),
	}

	subdir := ""
	if svc.Kustomize != nil {
		subdir = svc.Kustomize.Path
	}

	command := build.NewCmdBuild(filesys.MakeFsOnDisk(), help, out)
	dir := filepath.Join(k.dir, subdir)
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if ext := strings.ToLower(filepath.Ext(info.Name())); !lo.Contains([]string{".liquid"}, ext) {
			return nil
		}
		rpath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		data, err = renderLiquid(data, svc)
		if err != nil {
			return fmt.Errorf("templating error in %s: %w", rpath, err)
		}
		if err := writeFile(path, data); err != nil {
			return err
		}
		newPath := strings.TrimSuffix(path, ".liquid")
		return os.Rename(path, newPath)
	}); err != nil {
		return nil, err
	}

	command.SetArgs([]string{dir})
	if err := command.Execute(); err != nil {
		return nil, err
	}

	r := bytes.NewReader(out.Bytes())

	mapper, err := utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	readerOptions := manifestreader.ReaderOptions{
		Mapper:           mapper,
		Namespace:        svc.Namespace,
		EnforceNamespace: false,
	}
	mReader := &manifestreader.StreamManifestReader{
		ReaderName:    "kustomize",
		Reader:        r,
		ReaderOptions: readerOptions,
	}

	items, err := mReader.Read()
	if err != nil {
		return nil, err
	}
	return items, nil
}

func writeFile(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	return os.WriteFile(name, content, 0644)
}
