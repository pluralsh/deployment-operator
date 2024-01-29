package main

import (
	"flag"
	"os"
	"time"

	svccontroller "github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/samber/lo"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Logger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(deploymentsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

type controllerRunOptions struct {
	enableLeaderElection bool
	metricsAddr          string
	probeAddr            string
	refreshInterval      string
	processingTimeout    string
	resyncSeconds        int
	maxCurrentReconciles int
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
	flag.IntVar(&opt.maxCurrentReconciles, "max-current-reconciles", 10, "Maximum number of concurrent reconciles which can be run.")
	flag.IntVar(&opt.resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	flag.StringVar(&opt.refreshInterval, "refresh-interval", "2m", "Refresh interval duration")
	flag.StringVar(&opt.processingTimeout, "processing-timeout", "1m", "Maximum amount of time to spend trying to process queue item")
	flag.StringVar(&opt.consoleUrl, "console-url", "", "the url of the console api to fetch services from")
	flag.StringVar(&opt.deployToken, "deploy-token", "", "the deploy token to auth to console api with")
	flag.StringVar(&opt.clusterId, "cluster-id", "", "the id of the cluster being connected to")
	flag.BoolVar(&service.Local, "local", false, "whether you're running the operator locally")
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

	config := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		LeaderElection:         opt.enableLeaderElection,
		LeaderElectionID:       "dep12loy45.plural.sh",
		HealthProbeBindAddress: opt.probeAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}
	ctx := ctrl.SetupSignalHandler()

	// Start deployment operator controller manager.
	manager, err := svccontroller.NewControllerManager(ctx, opt.maxCurrentReconciles, pTimeout, refresh, lo.ToPtr(true), opt.consoleUrl, opt.deployToken, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	serviceReconciler, err := service.NewServiceReconciler(manager.GetClient(), config, manager.Refresh, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create service reconciler")
		os.Exit(1)
	}
	manager.AddController(&svccontroller.Controller{
		Name:  "Service Controller",
		Do:    serviceReconciler,
		Queue: serviceReconciler.SvcQueue,
	})

	if err := manager.Start(); err != nil {
		setupLog.Error(err, "unable to start controller manager")
		os.Exit(1)
	}

	if err = (&controller.CustomHealthReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ServiceReconciler: serviceReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HealthConvert")
	}
	//+kubebuilder:scaffold:builder

	if err = mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
