package template

import (
	"bytes"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/kustomize/kustomize/v4/commands/build"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

type kustomize struct {
	dir string
}

func NewKustomize(dir string) Template {
	return &kustomize{dir}
}

func (k *kustomize) Render(svc *client.ServiceDeployment, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	out := &bytes.Buffer{}
	h := build.MakeHelp("plural", "kustomize")
	help := &build.Help{
		Use:     h.Use,
		Short:   i18n.T(h.Short),
		Long:    templates.LongDesc(i18n.T(h.Long)),
		Example: templates.Examples(i18n.T(h.Example)),
	}

	command := build.NewCmdBuild(filesys.MakeFsOnDisk(), help, out)
	path := filepath.Join(k.dir, svc.Kustomize.Path)
	command.SetArgs([]string{path})
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
