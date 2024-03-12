package template

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/fs"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/homedir"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"

	"github.com/samber/lo"
)

const (
	appNameLabel                   = "app.kubernetes.io/name"
	appInstanceLabel               = "app.kubernetes.io/instance"
	appManagedByLabel              = "app.kubernetes.io/managed-by"
	appManagedByHelm               = "Helm"
	helmChartLabel                 = "helm.sh/chart"
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
)

func init() {
	EnableHelmDependencyUpdate = false

	// setup helm cache directory.
	dir, err := os.MkdirTemp("", "repositories")
	if err != nil {
		log.Panic(err)
	}
	settings.RepositoryCache = dir
	settings.RepositoryConfig = path.Join(dir, "repositories.yaml")
	settings.KubeInsecureSkipTLSVerify = true
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

func (h *helm) Render(svc *console.GetServiceDeploymentForAgent_ServiceDeployment, utilFactory util.Factory) ([]*unstructured.Unstructured, error) {
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
		if err := h.dependencyUpdate(config, c.Dependencies); err != nil {
			return nil, err
		}
	}

	rel, err := h.templateHelm(config, svc.Name, svc.Namespace, values)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	_, err = fmt.Fprintln(&buffer, strings.TrimSpace(rel.Manifest))
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(buffer.Bytes())
	mapper, err := utilFactory.ToRESTMapper()
	if err != nil {
		return nil, err
	}

	manifests, err := streamManifests(r, mapper, "helm", svc.Namespace)
	if err != nil {
		return nil, err
	}

	for _, manifest := range manifests {
		// Set recommended Helm labels. See: https://helm.sh/docs/chart_best_practices/labels/.
		labels := manifest.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[appManagedByLabel] = appManagedByHelm
		if _, ok := labels[appInstanceLabel]; !ok {
			labels[appInstanceLabel] = rel.Name
		}
		if _, ok := labels[appNameLabel]; !ok {
			labels[appNameLabel] = rel.Chart.Name()
		}
		if _, ok := labels[helmChartLabel]; !ok && rel.Chart.Metadata != nil {
			labels[helmChartLabel] = rel.Chart.Name() + "-" + strings.ReplaceAll(rel.Chart.Metadata.Version, "+", "_")
		}
		manifest.SetLabels(labels)

		// Set the same annotations that would be set by Helm to add release tracking metadata to all resources.
		annotations := manifest.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[helmReleaseNameAnnotation] = rel.Name
		annotations[helmReleaseNamespaceAnnotation] = rel.Namespace
		manifest.SetAnnotations(annotations)
	}

	return manifests, nil
}

func (h *helm) values(svc *console.GetServiceDeploymentForAgent_ServiceDeployment) (map[string]interface{}, error) {
	currentMap, err := h.valuesFile(svc, "values.yaml.liquid")
	if err != nil {
		return currentMap, err
	}
	if svc.Helm != nil && svc.Helm.ValuesFiles != nil && len(svc.Helm.ValuesFiles) > 0 {
		for _, f := range svc.Helm.ValuesFiles {
			nextMap, err := h.valuesFile(svc, lo.FromPtr(f))
			if err != nil {
				return currentMap, err
			}
			currentMap = algorithms.Merge(currentMap, nextMap)
		}
	}

	overrides, err := h.valuesFile(svc, "values.yaml.static")
	if err != nil {
		return currentMap, nil
	}

	return algorithms.Merge(currentMap, overrides), nil
}

func (h *helm) valuesFile(svc *console.GetServiceDeploymentForAgent_ServiceDeployment, filename string) (map[string]interface{}, error) {
	filename = filepath.Join(h.dir, filename)
	currentMap := map[string]interface{}{}
	if fs.Exists(filename) {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(filename, ".liquid") {
			data, err = renderLiquid(data, svc)
			if err != nil {
				return nil, err
			}
		}
		if err := yaml.Unmarshal(data, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", filename)
		}
	}

	return currentMap, nil
}

func (h *helm) templateHelm(conf *action.Configuration, name, namespace string, values map[string]interface{}) (*release.Release, error) {
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

	return client.Run(chart, values)
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

func (h *helm) dependencyUpdate(conf *action.Configuration, dependencies []*chart.Dependency) error {

	for _, dep := range dependencies {
		if err := AddRepo(dep.Name, dep.Repository); err != nil {
			return err
		}
	}
	if err := UpdateRepos(); err != nil {
		return err
	}

	man := &downloader.Manager{
		Out:       log.Writer(),
		ChartPath: h.dir,
		Keyring:   defaultKeyring(),
		// Must be skipped. The updater searches for the repos in the local helm cache directory, not the /temp. Fails in container.
		SkipUpdate:       true,
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

var errNoRepositories = errors.New("no repositories found. You must add one before updating")

func UpdateRepos() error {
	log.Println("helm repo update...")
	f, err := repo.LoadFile(settings.RepositoryConfig)
	switch {
	case isNotExist(err):
		return errNoRepositories
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", settings.RepositoryConfig)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}
	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		r.CachePath = settings.RepositoryCache
		repos = append(repos, r)

	}

	return updateCharts(repos, true)

}

func updateCharts(repos []*repo.ChartRepository, failOnRepoUpdateFail bool) error {
	var wg sync.WaitGroup
	var repoFailList []string
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				log.Printf("unable to get an update from the %q chart repository (%s):\n\t%s\n", re.Config.Name, re.Config.URL, err)
				repoFailList = append(repoFailList, re.Config.URL)
			} else {
				log.Printf("successfully got an update from the %q chart repository\n", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()

	if len(repoFailList) > 0 && failOnRepoUpdateFail {
		return fmt.Errorf("failed to update the following repositories: %s",
			repoFailList)
	}

	log.Printf("Update Complete.")
	return nil
}

func AddRepo(repoName, repoUrl string) error {
	repoFile := settings.RepositoryConfig
	err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}
	log.Printf("Adding repo %s to %s\n", repoName, repoFile)
	// Acquire a file lock for process synchronization.
	repoFileExt := filepath.Ext(repoFile)
	var lockPath string
	if len(repoFileExt) > 0 && len(repoFileExt) < len(repoFile) {
		lockPath = strings.TrimSuffix(repoFile, repoFileExt) + ".lock"
	} else {
		lockPath = repoFile + ".lock"
	}
	fileLock := flock.New(lockPath)
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer func(fileLock *flock.Flock) {
			_ = fileLock.Unlock()
		}(fileLock)
	}
	if err != nil {
		return err
	}

	b, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	c := repo.Entry{
		Name:                  repoName,
		URL:                   repoUrl,
		InsecureSkipTLSverify: true,
	}
	f.Update(&c)
	log.Printf("Repo %s added to %s\n", repoName, repoFile)
	return f.WriteFile(repoFile, 0644)
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}
