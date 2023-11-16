package main

import (
	"flag"
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/agent"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
	"github.com/pluralsh/deployment-operator/pkg/sync"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Logger
)

type controllerRunOptions struct {
	enableLeaderElection bool
	metricsAddr          string
	probeAddr            string
	refreshInterval      string
	processingTimeout    string
	resyncSeconds        int
	consoleUrl           string
	deployToken          string
	clusterId            string
}

func main() {
	klog.InitFlags(nil)

	opt := &controllerRunOptions{}
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.StringVar(&opt.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&opt.probeAddr, "health-probe-bind-address", ":9001", "The address the probe endpoint binds to.")
	flag.BoolVar(&opt.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&opt.resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	flag.StringVar(&opt.refreshInterval, "refresh-interval", "2m", "Refresh interval duration")
	flag.StringVar(&opt.processingTimeout, "processing-timeout", "1m", "Maximum amount of time to spend trying to process queue item")
	flag.StringVar(&opt.consoleUrl, "console-url", "", "the url of the console api to fetch services from")
	flag.StringVar(&opt.deployToken, "deploy-token", "", "the deploy token to auth to console api with")
	flag.StringVar(&opt.clusterId, "cluster-id", "", "the id of the cluster being connected to")
	flag.BoolVar(&sync.Local, "local", false, "whether you're running the operator locally")
	flag.BoolVar(&template.EnableHelmDependencyUpdate, "enable-helm-dependency-update", false, "enable update helm chart's dependencies")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if opt.deployToken == "" {
		opt.deployToken = os.Getenv("DEPLOY_TOKEN")
	}
	refresh, err := time.ParseDuration(opt.refreshInterval)
	if err != nil {
		setupLog.Error(err, "unable to get refresh interval")
		os.Exit(1)
	}

	pTimeout, err := time.ParseDuration(opt.processingTimeout)
	if err != nil {
		setupLog.Error(err, "unable to get processing timeout")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         opt.enableLeaderElection,
		LeaderElectionID:       "dep12loy45.plural.sh",
		HealthProbeBindAddress: opt.probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}
	if err = mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	a, err := agent.New(mgr.GetConfig(), refresh, pTimeout, opt.consoleUrl, opt.deployToken, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create agent")
		os.Exit(1)
	}
	if err := a.SetupWithManager(); err != nil {
		setupLog.Error(err, "unable to start agent")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
