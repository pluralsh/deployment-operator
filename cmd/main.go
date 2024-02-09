package main

import (
	"os"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	pipelinecontroller "github.com/pluralsh/deployment-operator/controllers/pipelinegates"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/pkg/client"
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
	utilruntime.Must(pipelinesv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	opt := newOptions()
	config := ctrl.GetConfigOrDie()
	ctx := ctrl.SetupSignalHandler()

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

	setupLog.Info("starting agent")
	//ctrlMgr, serviceReconciler := runAgent(opt, config, ctx, mgr.GetClient())
	_, serviceReconciler := runAgent(opt, config, ctx, mgr.GetClient())

	//if err = (&controller.BackupReconciler{
	//	Client:        mgr.GetClient(),
	//	Scheme:        mgr.GetScheme(),
	//	ConsoleClient: ctrlMgr.GetClient(),
	//}).SetupWithManager(mgr); err != nil {
	//	setupLog.Error(err, "unable to create controller", "controller", "Backup")
	//}

	if err = (&controller.CustomHealthReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ServiceReconciler: serviceReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HealthConvert")
	}
	//+kubebuilder:scaffold:builder

	if err = (&pipelinecontroller.PipelineGateReconciler{
		Client:        mgr.GetClient(),
		ConsoleClient: client.New(opt.consoleUrl, opt.deployToken),
		Log:           ctrl.Log.WithName("controllers").WithName("PipelineGate"),
		Scheme:        mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Group")
		os.Exit(1)
	}

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
