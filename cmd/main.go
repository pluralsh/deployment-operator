package main

import (
	"flag"
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/agent"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Logger
)

func main() {
	var enableLeaderElection bool
	var metricsAddr string
	var probeAddr string
	var refreshInterval string
	var resyncSeconds int
	var consoleUrl string
	var deployToken string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":9001", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	flag.StringVar(&refreshInterval, "refresh-interval", "1m", "Refresh interval duration")
	flag.StringVar(&consoleUrl, "console-url", "", "the url of the console api to fetch services from")
	flag.StringVar(&deployToken, "deploy-token", "", "the deploy token to auth to console api with")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if deployToken == "" {
		deployToken = os.Getenv("DEPLOY_TOKEN")
	}
	refresh, err := time.ParseDuration(refreshInterval)
	if err != nil {
		setupLog.Error(err, "unable to get refresh interval")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "dep12loy45.plural.sh",
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}
	if err = mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	a, err := agent.New(mgr.GetConfig(), refresh, consoleUrl, deployToken)
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
