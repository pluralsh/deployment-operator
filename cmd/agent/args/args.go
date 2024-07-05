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

	defaultResourceCacheTTL         = "1h"
	defaultResourceCacheTTLDuration = time.Hour

	defaultRestoreNamespace = "velero"
)

var (
	argDisableHelmTemplateDryRunServer = flag.Bool("disable-helm-dry-run-server", false, "Disable helm template in dry-run=server mode.")
	argEnableHelmDependencyUpdate      = flag.Bool("enable-helm-dependency-update", false, "Enable update Helm chart's dependencies.")
	argEnableLeaderElection            = flag.Bool("leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	argLocal                           = flag.Bool("local", false, "Whether you're running the operator locally.")

	argMaxConcurrentReconciles = flag.Int("max-concurrent-reconciles", 20, "Maximum number of concurrent reconciles which can be run.")
	argResyncSeconds           = flag.Int("resync-seconds", 300, "Resync duration in seconds.")

	argClusterId         = flag.String("cluster-id", "", "The ID of the cluster being connected to.")
	argConsoleUrl        = flag.String("console-url", "", "The URL of the console api to fetch services from.")
	argDeployToken       = flag.String("deploy-token", helpers.GetEnv(EnvDeployToken, ""), "The deploy token to auth to Console API with.")
	argProbeAddr         = flag.String("health-probe-bind-address", defaultProbeAddress, "The address the probe endpoint binds to.")
	argMetricsAddr       = flag.String("metrics-bind-address", defaultMetricsAddress, "The address the metric endpoint binds to.")
	argProcessingTimeout = flag.String("processing-timeout", defaultProcessingTimeout, "Maximum amount of time to spend trying to process queue item.")
	argRefreshInterval   = flag.String("refresh-interval", defaultRefreshInterval, "Refresh interval duration.")
	argResourceCacheTTL  = flag.String("resource-cache-ttl", defaultResourceCacheTTL, "The time to live of each resource cache entry.")
	argRestoreNamespace  = flag.String("restore-namespace", defaultRestoreNamespace, "The namespace where Velero restores are located.")
	argServices          = flag.String("services", "", "A comma separated list of service ids to reconcile. Leave empty to reconcile all.")

	serviceSet containers.Set[string]
)

func Init() {
	// Init klog
	fs := flag.NewFlagSet("", flag.PanicOnError)
	klog.InitFlags(fs)

	// Use default log level defined by the application
	_ = fs.Set("v", fmt.Sprintf("%d", log.LogLevelDefault))

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)

	pflag.CommandLine.AddGoFlagSet(fs)
	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		flag.CommandLine.Var(f.Value, f.Name, f.Usage)
	})
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Initialize unique service set
	if len(*argServices) > 0 {
		serviceSet = containers.ToSet(strings.Split(*argServices, ","))
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
		klog.ErrorS(err, "Could not parse refresh-interval", "value", *argProcessingTimeout, "default", defaultRefreshIntervalDuration)
		return defaultRefreshIntervalDuration
	}

	return duration
}

func ResourceCacheTTL() time.Duration {
	duration, err := time.ParseDuration(*argResourceCacheTTL)
	if err != nil {
		klog.ErrorS(err, "Could not parse resource-cache-ttl", "value", *argResourceCacheTTL, "default", defaultResourceCacheTTLDuration)
		return defaultResourceCacheTTLDuration
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

func ensureOrDie(argName string, arg *string) {
	if arg == nil || len(*arg) == 0 {
		pflag.PrintDefaults()
		panic(fmt.Sprintf("%s arg is rquired", argName))
	}
}
