package helm

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
)

type Helm struct {
	// configuration ...
	configuration *action.Configuration

	// chart ...
	chart *chart.Chart

	// kubeconfig ...
	kubeconfig *string

	// chartName ...
	chartName string

	// releaseName
	releaseName string

	// releaseNamespace ...
	releaseNamespace string

	// repository ...
	repository string

	// values ...
	values map[string]interface{}
}

func (in *Helm) Install() error {
	installAction := action.NewInstall(in.configuration)

	// Action config
	installAction.Namespace = in.releaseNamespace
	installAction.ReleaseName = in.releaseName
	installAction.Timeout = 5 * time.Minute
	installAction.Wait = true
	installAction.CreateNamespace = true

	_, err := installAction.Run(in.chart, in.values)
	return err
}

func (in *Helm) Upgrade(install bool) error {
	installed, err := in.installed()
	if err != nil {
		return err
	}

	if !installed && !install {
		return fmt.Errorf("no helm chart installation found")
	}

	if !installed {
		return in.Install()
	}

	upgradeAction := action.NewUpgrade(in.configuration)

	// Action config
	upgradeAction.Namespace = in.releaseNamespace
	upgradeAction.Timeout = 5 * time.Minute
	upgradeAction.Wait = false

	_, err = upgradeAction.Run(in.releaseName, in.chart, in.values)
	return nil
}

// releases lists all releases that match the given state.
func (in *Helm) releases(state action.ListStates, config *action.Configuration) ([]*release.Release, error) {
	listAction := action.NewList(config)
	listAction.StateMask = state

	return listAction.Run()
}

func (in *Helm) installed() (bool, error) {
	releases, err := in.releases(action.ListAll, in.configuration)
	if err != nil {
		return false, err
	}

	for _, r := range releases {
		if r.Name == in.releaseName && r.Namespace == in.releaseNamespace {
			return true, nil
		}
	}

	return false, nil
}

func (in *Helm) restConfig() (*rest.Config, error) {
	if in.kubeconfig != nil {
		return clientcmd.RESTConfigFromKubeConfig([]byte(*in.kubeconfig))
	}

	return config.GetConfig()
}

func (in *Helm) init() (*Helm, error) {
	if len(in.releaseName) == 0 {
		return in, fmt.Errorf("releaseName cannot be empty")
	}

	if len(in.chartName) == 0 {
		return in, fmt.Errorf("chartName cannot be empty")
	}

	if len(in.repository) == 0 {
		return in, fmt.Errorf("repository cannot be empty")
	}

	if len(in.releaseNamespace) == 0 {
		in.releaseNamespace = "default"
	}

	if err := in.initRepo(); err != nil {
		return in, err
	}

	if err := in.initConfiguration(); err != nil {
		return in, err
	}

	if err := in.initChart(); err != nil {
		return in, err
	}

	return in, nil
}

func (in *Helm) initRepo() error {
	return template.AddRepo(in.releaseName, in.repository)
}

func (in *Helm) initConfiguration() error {
	in.configuration = new(action.Configuration)
	restConfig, err := in.restConfig()
	if err != nil {
		return err
	}

	return in.configuration.Init(utils.NewFactory(restConfig), in.releaseNamespace, "", logrus.Debugf)
}

func (in *Helm) initChart() error {
	installAction := action.NewInstall(in.configuration)
	locateName := fmt.Sprintf("%s/%s", in.releaseName, in.chartName)
	path, err := installAction.ChartPathOptions.LocateChart(locateName, cli.New())
	if err != nil {
		return err
	}

	_, err = loader.Load(path)
	return err
}

// New ...
func New(options ...Option) (*Helm, error) {
	result := &Helm{}

	for _, option := range options {
		option(result)
	}

	return result.init()
}
