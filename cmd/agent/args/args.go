package args

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pluralsh/polly/containers"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	EnvDeployToken = "DEPLOY_TOKEN"

	defaultProbeAddress   = ":9001"
	defaultMetricsAddress = ":8000"

	defaultProcessingTimeout         = "1m"
	defaultProcessingTimeoutDuration = time.Minute

	defaultRefreshInterval         = "2m"
	defaultRefreshIntervalDuration = 2 * time.Minute

	defaultPollInterval         = "30s"
	defaultPollIntervalDuration = 30 * time.Second

	defaultPollJitter         = "15s"
	defaultPollJitterDuration = 15 * time.Second

	defaultResourceCacheTTL         = "1h"
	defaultResourceCacheTTLDuration = time.Hour

	defaultManifestCacheTTL         = "1h"
	defaultManifestCacheTTLDuration = time.Hour

	defaultControllerCacheTTL         = "30s"
	defaultControllerCacheTTLDuration = 30 * time.Second

	defaultRestoreNamespace = "velero"

	defaultProfilerPath    = "/debug/pprof/"
	defaultProfilerAddress = ":7777"
)

var (
	argDisableHelmTemplateDryRunServer = flag.Bool("disable-helm-dry-run-server", false, "Disable helm template in dry-run=server mode.")
	argEnableHelmDependencyUpdate      = flag.Bool("enable-helm-dependency-update", false, "Enable update Helm chart's dependencies.")
	argEnableLeaderElection            = flag.Bool("leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	argLocal                           = flag.Bool("local", false, "Whether you're running the operator locally.")
	argProfiler                        = flag.Bool("profiler", false, "Enable pprof handler. By default it will be exposed on localhost:7777 under '/debug/pprof'")
	argDisableResourceCache            = flag.Bool("disable-resource-cache", false, "Control whether resource cache should be enabled or not.")

	argMaxConcurrentReconciles = flag.Int("max-concurrent-reconciles", 20, "Maximum number of concurrent reconciles which can be run.")
	argResyncSeconds           = flag.Int("resync-seconds", 300, "Resync duration in seconds.")

	argClusterId         = flag.String("cluster-id", "", "The ID of the cluster being connected to.")
	argConsoleUrl        = flag.String("console-url", "", "The URL of the console api to fetch services from.")
	argDeployToken       = flag.String("deploy-token", helpers.GetEnv(EnvDeployToken, ""), "The deploy token to auth to Console API with.")
	argProbeAddr         = flag.String("health-probe-bind-address", defaultProbeAddress, "The address the probe endpoint binds to.")
	argMetricsAddr       = flag.String("metrics-bind-address", defaultMetricsAddress, "The address the metric endpoint binds to.")
	argProcessingTimeout = flag.String("processing-timeout", defaultProcessingTimeout, "Maximum amount of time to spend trying to process queue item.")
	argRefreshInterval   = flag.String("refresh-interval", defaultRefreshInterval, "Time interval to recheck the websocket connection.")
	argPollInterval      = flag.String("poll-interval", defaultPollInterval, "Time interval to poll resources from the Console API.")
	// TODO: ensure this arg can be safely renamed without causing breaking changes.
	argPollJitter         = flag.String("refresh-jitter", defaultPollJitter, "Randomly selected jitter time up to the provided duration will be added to the poll interval.")
	argResourceCacheTTL   = flag.String("resource-cache-ttl", defaultResourceCacheTTL, "The time to live of each resource cache entry.")
	argManifestCacheTTL   = flag.String("manifest-cache-ttl", defaultManifestCacheTTL, "The time to live of service manifests in cache entry.")
	argControllerCacheTTL = flag.String("controller-cache-ttl", defaultControllerCacheTTL, "The time to live of console controller cache entries.")
	argRestoreNamespace   = flag.String("restore-namespace", defaultRestoreNamespace, "The namespace where Velero restores are located.")
	argServices           = flag.String("services", "", "A comma separated list of service ids to reconcile. Leave empty to reconcile all.")

	serviceSet containers.Set[string]
)

func Init() {
	defaultFlagSet := flag.CommandLine

	// Init klog
	klog.InitFlags(defaultFlagSet)

	// Use default log level defined by the application
	_ = defaultFlagSet.Set("v", fmt.Sprintf("%d", log.LogLevelDefault))

	opts := zap.Options{Development: true}
	opts.BindFlags(defaultFlagSet)

	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Initialize unique service set
	if len(*argServices) > 0 {
		serviceSet = containers.ToSet(strings.Split(*argServices, ","))
	}

	if *argProfiler {
		initProfiler()
	}

	klog.V(log.LogLevelMinimal).InfoS("configured log level", "v", LogLevel())
}

func DisableHelmTemplateDryRunServer() bool {
	return *argDisableHelmTemplateDryRunServer
}

func EnableHelmDependencyUpdate() bool {
	return *argEnableHelmDependencyUpdate
}

func EnableLeaderElection() bool {
	return *argEnableLeaderElection
}

func Local() bool {
	return *argLocal
}

func MaxConcurrentReconciles() int {
	return *argMaxConcurrentReconciles
}

func ResyncSeconds() int {
	return *argResyncSeconds
}

func ClusterId() string {
	ensureOrDie("cluster-id", argClusterId)

	return *argClusterId
}

func ConsoleUrl() string {
	ensureOrDie("console-url", argConsoleUrl)

	return *argConsoleUrl
}

func DeployToken() string {
	ensureOrDie("deploy-token", argDeployToken)

	return *argDeployToken
}

func ProbeAddr() string {
	return *argProbeAddr
}

func MetricsAddr() string {
	return *argMetricsAddr
}

func ProcessingTimeout() time.Duration {
	duration, err := time.ParseDuration(*argProcessingTimeout)
	if err != nil {
		klog.ErrorS(err, "Could not parse processing-timeout", "value", *argProcessingTimeout, "default", defaultProcessingTimeoutDuration)
		return defaultProcessingTimeoutDuration
	}

	return duration
}

func RefreshInterval() time.Duration {
	duration, err := time.ParseDuration(*argRefreshInterval)
	if err != nil {
		klog.ErrorS(err, "Could not parse refresh-interval", "value", *argRefreshInterval, "default", defaultRefreshIntervalDuration)
		return defaultRefreshIntervalDuration
	}

	return duration
}

func PollInterval() time.Duration {
	duration, err := time.ParseDuration(*argPollInterval)
	if err != nil {
		klog.ErrorS(err, "Could not parse poll-interval", "value", *argPollInterval, "default", defaultPollIntervalDuration)
		return defaultPollIntervalDuration
	}

	if duration < 10*time.Second {
		klog.Fatalf("--poll-interval cannot be lower than 10s")
	}

	return duration
}

func PollJitter() time.Duration {
	jitter, err := time.ParseDuration(*argPollJitter)
	if err != nil {
		klog.ErrorS(err, "Could not parse refresh-jitter", "value", *argPollJitter, "default", defaultPollJitterDuration)
		return defaultPollJitterDuration
	}

	if jitter < 10*time.Second {
		klog.Fatalf("--refresh-jitter cannot be lower than 10s")
	}

	return jitter
}

func ResourceCacheTTL() time.Duration {
	duration, err := time.ParseDuration(*argResourceCacheTTL)
	if err != nil {
		klog.ErrorS(err, "Could not parse resource-cache-ttl", "value", *argResourceCacheTTL, "default", defaultResourceCacheTTLDuration)
		return defaultResourceCacheTTLDuration
	}

	return duration
}

func ManifestCacheTTL() time.Duration {
	duration, err := time.ParseDuration(*argManifestCacheTTL)
	if err != nil {
		klog.ErrorS(err, "Could not parse manifest-cache-ttl", "value", *argManifestCacheTTL, "default", defaultManifestCacheTTLDuration)
		return defaultManifestCacheTTLDuration
	}

	return duration
}

func ControllerCacheTTL() time.Duration {
	duration, err := time.ParseDuration(*argControllerCacheTTL)
	if err != nil {
		klog.ErrorS(err, "Could not parse controller-cache-ttl", "value", *argControllerCacheTTL, "default", defaultControllerCacheTTLDuration)
		return defaultControllerCacheTTLDuration
	}

	return duration
}

func RestoreNamespace() string {
	return *argRestoreNamespace
}

func SkipService(id string) bool {
	return serviceSet.Len() > 0 && !serviceSet.Has(id)
}

func LogLevel() klog.Level {
	v := pflag.Lookup("v")
	if v == nil {
		return log.LogLevelDefault
	}

	level, err := strconv.ParseInt(v.Value.String(), 10, 32)
	if err != nil {
		klog.ErrorS(err, "Could not parse log level", "level", v.Value.String(), "default", log.LogLevelDefault)
		return log.LogLevelDefault
	}

	return klog.Level(level)
}

func ResourceCacheEnabled() bool {
	return !(*argDisableResourceCache)
}

func ensureOrDie(argName string, arg *string) {
	if arg == nil || len(*arg) == 0 {
		pflag.PrintDefaults()
		panic(fmt.Sprintf("%s arg is rquired", argName))
	}
}
