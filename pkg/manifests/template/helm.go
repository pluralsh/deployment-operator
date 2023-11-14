package template

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/fs"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/manifestreader"
	"sigs.k8s.io/yaml"
)

func init() {
	EnableHelmDependencyUpdate = false
}

var settings = cli.New()
var EnableHelmDependencyUpdate bool

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("INFO: %s\n", format)
	err := log.Output(2, fmt.Sprintf(format, v...))
	if err != nil {
		log.Panic(err)
	}
}

type helm struct {
	dir string
}

func (h *helm) Render(svc *console.ServiceDeploymentExtended, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
	// TODO: add some configured values file convention, perhaps using our lua templating from plural-cli
	values, err := h.values(svc)
	if err != nil {
		return nil, err
	}
	config, err := GetActionConfig(svc.Namespace)
	if err != nil {
		return nil, err
	}
	c, err := chartutil.LoadChartfile(path.Join(h.dir, ChartFileName))
	if err != nil {
		return nil, err
	}

	log.Println("render helm templates:", "enable dependency update=", EnableHelmDependencyUpdate, "dependencies=", len(c.Dependencies))
	if len(c.Dependencies) > 0 && EnableHelmDependencyUpdate {
		if err := h.dependencyUpdate(config); err != nil {
			return nil, err
		}
	}

	out, err := h.templateHelm(config, svc.Name, svc.Namespace, values)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(out)

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

func (h *helm) values(svc *console.ServiceDeploymentExtended) (map[string]interface{}, error) {
	lqPath := filepath.Join(h.dir, "values.yaml.liquid")
	currentMap := map[string]interface{}{}
	if fs.Exists(lqPath) {
		data, err := os.ReadFile(lqPath)
		if err != nil {
			return nil, err
		}

		data, err = renderLiquid(data, svc)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", lqPath)
		}
	}

	return currentMap, nil
}

func (h *helm) templateHelm(conf *action.Configuration, name, namespace string, values map[string]interface{}) ([]byte, error) {
	// load chart from the path
	chart, err := loader.Load(h.dir)
	if err != nil {
		return nil, err
	}

	client := action.NewInstall(conf)
	client.DryRun = true
	client.ReleaseName = name
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.Namespace = namespace
	client.IncludeCRDs = true
	client.IsUpgrade = true
	vsn, err := kubeVersion(conf)
	if err != nil {
		return nil, err
	}
	client.KubeVersion = vsn

	rel, err := client.Run(chart, values)
	if err != nil {
		return nil, err
	}
	var manifests bytes.Buffer
	_, err = fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	if err != nil {
		return nil, err
	}
	return manifests.Bytes(), nil
}

func GetActionConfig(namespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	if os.Getenv("KUBECONFIG") != "" {
		settings.KubeConfig = os.Getenv("KUBECONFIG")
	}

	settings.SetNamespace(namespace)
	settings.Debug = false
	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, "", debug); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

func NewHelm(dir string) Template {
	return &helm{dir}
}

func kubeVersion(conf *action.Configuration) (*chartutil.KubeVersion, error) {
	dc, err := conf.RESTClientGetter.ToDiscoveryClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not get Kubernetes discovery client")
	}

	kubeVersion, err := dc.ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "could not get server version from Kubernetes")
	}

	return &chartutil.KubeVersion{
		Version: kubeVersion.GitVersion,
		Major:   kubeVersion.Major,
		Minor:   kubeVersion.Minor,
	}, nil
}

func (h *helm) dependencyUpdate(conf *action.Configuration) error {
	man := &downloader.Manager{
		Out:              log.Writer(),
		ChartPath:        h.dir,
		Keyring:          defaultKeyring(),
		SkipUpdate:       false,
		Getters:          getter.All(settings),
		RegistryClient:   conf.RegistryClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		Debug:            false,
	}
	return man.Update()
}

// defaultKeyring returns the expanded path to the default keyring.
func defaultKeyring() string {
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		return filepath.Join(v, "pubring.gpg")
	}
	return filepath.Join(homedir.HomeDir(), ".gnupg", "pubring.gpg")
}
