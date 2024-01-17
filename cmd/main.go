package main

import (
	"os"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/pkg/log"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = log.Logger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deploymentsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	opt := newOptions()
	config := ctrl.GetConfigOrDie()
	ctx := ctrl.SetupSignalHandler()

	ctrlMgr, serviceReconciler := runAgent(opt, config, ctx)

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

	if err = (&controller.RestoreReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConsoleClient: ctrlMgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Restore")
	}
	if err = (&controller.ClusterBackupReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConsoleClient: ctrlMgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Backup")
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

	a, err := agent.New(mgr, refresh, pTimeout, opt.consoleUrl, opt.deployToken, opt.clusterId)
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
