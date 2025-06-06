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

	"github.com/pluralsh/polly/luautils"
	lua "github.com/yuin/gopher-lua"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/fs"
	"github.com/samber/lo"
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
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	loglevel "github.com/pluralsh/deployment-operator/pkg/log"
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

func (h *helm) Render(svc *console.ServiceDeploymentForAgent, utilFactory util.Factory) ([]unstructured.Unstructured, error) {
	values, err := h.values(svc)
	if err != nil {
		return nil, err
	}
	luaValues, err := h.luaValues(svc)
	if err != nil {
		return nil, err
	}
	values = algorithms.Merge(values, luaValues)

	config, err := GetActionConfig(svc.Namespace)
	if err != nil {
		return nil, err
	}
	c, err := chartutil.LoadChartfile(path.Join(h.dir, ChartFileName))
	if err != nil {
		return nil, err
	}

	klog.V(loglevel.LogLevelExtended).InfoS("render helm templates:", "enable dependency update", args.EnableHelmDependencyUpdate(), "dependencies", len(c.Dependencies))
	if len(c.Dependencies) > 0 && args.EnableHelmDependencyUpdate() {
		if err := h.dependencyUpdate(config, c.Dependencies); err != nil {
			return nil, err
		}
	}

	release := svc.Name
	if svc.Helm != nil && svc.Helm.Release != nil {
		release = *svc.Helm.Release
	}

	includeCRDs := true
	if svc.Helm != nil && svc.Helm.IgnoreCrds != nil {
		includeCRDs = !*svc.Helm.IgnoreCrds
	}

	rel, err := h.templateHelm(config, release, svc.Namespace, values, includeCRDs)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	_, err = fmt.Fprintln(&buffer, strings.TrimSpace(rel.Manifest))
	if err != nil {
		return nil, err
	}
	if svc.Helm != nil && svc.Helm.IgnoreHooks != nil && !*svc.Helm.IgnoreHooks {
		for _, h := range rel.Hooks {
			_, err = fmt.Fprintln(&buffer, "---")
			if err != nil {
				return nil, err
			}
			_, err = fmt.Fprintln(&buffer, strings.TrimSpace(h.Manifest))
			if err != nil {
				return nil, err
			}
		}
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

func (h *helm) luaValues(svc *console.ServiceDeploymentForAgent) (map[string]interface{}, error) {
	// Initialize empty results
	newValues := make(map[string]interface{})
	var valuesFiles []string

	if svc == nil {
		return nil, fmt.Errorf("no service found")
	}
	if svc.Helm == nil {
		return newValues, nil
	}
	if svc.Helm.LuaScript == nil {
		return newValues, nil
	}
	p := luautils.NewProcessor(h.dir)
	defer p.L.Close()

	// Register global values and valuesFiles in Lua
	valuesTable := p.L.NewTable()
	p.L.SetGlobal("values", valuesTable)

	valuesFilesTable := p.L.NewTable()
	p.L.SetGlobal("valuesFiles", valuesFilesTable)

	// Execute the Lua script
	err := p.L.DoString(*svc.Helm.LuaScript)
	if err != nil {
		return nil, err
	}

	if err := luautils.MapLua(p.L.GetGlobal("values").(*lua.LTable), &newValues); err != nil {
		return nil, err
	}

	if err := luautils.MapLua(p.L.GetGlobal("valuesFiles").(*lua.LTable), &valuesFiles); err != nil {
		return nil, err
	}

	for _, file := range valuesFiles {
		currentMap, err := h.valuesFile(svc, file)
		if err == nil {
			newValues = algorithms.Merge(newValues, currentMap)
		}
	}

	return newValues, nil
}

func (h *helm) values(svc *console.ServiceDeploymentForAgent) (map[string]interface{}, error) {
	currentMap, err := h.valuesFile(svc, "values.yaml.liquid")
	if err != nil {
		return currentMap, err
	}
	if svc.Helm != nil {
		for _, f := range svc.Helm.ValuesFiles {
			nextMap, err := h.valuesFile(svc, lo.FromPtr(f))
			if err != nil {
				return currentMap, err
			}
			currentMap = algorithms.Merge(currentMap, nextMap)
		}

		if svc.Helm.Values != nil {
			valuesMap := map[string]interface{}{}
			if err := yaml.Unmarshal([]byte(*svc.Helm.Values), &valuesMap); err != nil {
				return nil, err
			}
			currentMap = algorithms.Merge(currentMap, valuesMap)
		}
	}

	overrides, err := h.valuesFile(svc, "values.yaml.static")
	if err != nil {
		return currentMap, nil
	}

	return algorithms.Merge(currentMap, overrides), nil
}

func (h *helm) valuesFile(svc *console.ServiceDeploymentForAgent, filename string) (map[string]interface{}, error) {
	filename = filepath.Join(h.dir, filename)
	currentMap := map[string]interface{}{}
	if fs.Exists(filename) {
		data, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(filename, ".liquid") {
			data, err = renderLiquid(data, svc)
		}

		if strings.HasSuffix(filename, ".tpl") {
			data, err = renderTpl(data, svc)
		}

		if err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(data, &currentMap); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", filename)
		}
	}

	return currentMap, nil
}

func (h *helm) templateHelm(conf *action.Configuration, release, namespace string, values map[string]any, includeCRDs bool) (*release.Release, error) {
	// load chart from the path
	chart, err := loader.Load(h.dir)
	if err != nil {
		return nil, err
	}

	client := action.NewInstall(conf)
	client.DryRun = true
	if !args.DisableHelmTemplateDryRunServer() {
		client.DryRunOption = "server"
	}
	client.ReleaseName = release
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.Namespace = namespace
	client.IncludeCRDs = includeCRDs
	client.IsUpgrade = true
	vsn, err := kubeVersion(conf)
	if err != nil {
		return nil, err
	}
	client.KubeVersion = vsn
	client.APIVersions = algorithms.MapKeys[string, bool](cache.DiscoveryCache().Items())

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
	klog.V(loglevel.LogLevelExtended).InfoS("helm repo update...")
	f, err := repo.LoadFile(settings.RepositoryConfig)
	switch {
	case isNotExist(err):
		return errNoRepositories
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", settings.RepositoryConfig)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}
	repos := make([]repo.ChartRepository, 0, len(f.Repositories))
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		r.CachePath = settings.RepositoryCache
		repos = append(repos, *r)
	}

	return updateCharts(repos, true)
}

func updateCharts(repos []repo.ChartRepository, failOnRepoUpdateFail bool) error {
	var wg sync.WaitGroup
	var repoFailList []string
	for _, re := range repos {
		wg.Add(1)
		go func(re repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				klog.ErrorS(err, "unable to get an update from the chart repository", "name", re.Config.Name, "url", re.Config.URL)
				repoFailList = append(repoFailList, re.Config.URL)
			} else {
				klog.V(loglevel.LogLevelExtended).InfoS("successfully got an update from the chart repository", "name", re.Config.Name)
			}
		}(re)
	}
	wg.Wait()

	if len(repoFailList) > 0 && failOnRepoUpdateFail {
		return fmt.Errorf("failed to update the following repositories: %s",
			repoFailList)
	}

	klog.V(loglevel.LogLevelExtended).InfoS("helm repo update complete")
	return nil
}

func AddRepo(repoName, repoUrl string) error {
	repoFile := settings.RepositoryConfig
	err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}
	klog.V(loglevel.LogLevelExtended).InfoS("adding helm repo", "name", repoName, "file", repoFile)
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
	klog.V(loglevel.LogLevelExtended).InfoS("helm repo added", "name", repoName, "file", repoFile)
	return f.WriteFile(repoFile, 0644)
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

func HelmSettings() *cli.EnvSettings {
	return settings
}
