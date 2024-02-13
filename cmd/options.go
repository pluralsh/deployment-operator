package main

import (
	"flag"
	"os"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/pluralsh/deployment-operator/pkg/manifests/template"

	"github.com/pluralsh/deployment-operator/pkg/controller/service"
)

type options struct {
	enableLeaderElection    bool
	metricsAddr             string
	probeAddr               string
	refreshInterval         string
	processingTimeout       string
	resyncSeconds           int
	maxConcurrentReconciles int
	consoleUrl              string
	deployToken             string
	clusterId               string
	restoreNamespace        string
}

func newOptions() *options {
	klog.InitFlags(nil)

	o := &options{}

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)

	flag.StringVar(&o.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&o.probeAddr, "health-probe-bind-address", ":9001", "The address the probe endpoint binds to.")
	flag.BoolVar(&o.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&o.maxConcurrentReconciles, "max-concurrent-reconciles", 10, "Maximum number of concurrent reconciles which can be run.")
	flag.IntVar(&o.resyncSeconds, "resync-seconds", 300, "Resync duration in seconds.")
	flag.StringVar(&o.refreshInterval, "refresh-interval", "2m", "Refresh interval duration.")
	flag.StringVar(&o.processingTimeout, "processing-timeout", "1m", "Maximum amount of time to spend trying to process queue item.")
	flag.StringVar(&o.consoleUrl, "console-url", "", "The URL of the console api to fetch services from.")
	flag.StringVar(&o.deployToken, "deploy-token", "", "The deploy token to auth to Console API with.")
	flag.StringVar(&o.clusterId, "cluster-id", "", "The ID of the cluster being connected to.")
	flag.StringVar(&o.restoreNamespace, "restore-namespace", "velero", "The namespace where Velero restores are located.")
	flag.BoolVar(&service.Local, "local", false, "Whether you're running the operator locally.")
	flag.BoolVar(&template.EnableHelmDependencyUpdate, "enable-helm-dependency-update", false, "Enable update Helm chart's dependencies.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if o.deployToken == "" {
		o.deployToken = os.Getenv("DEPLOY_TOKEN")
	}

	return o
}
