package main

import (
	"flag"
	"os"

	platform "github.com/pluralsh/deployment-api/apis/platform/v1alpha1"
	deploymentrequestpackage "github.com/pluralsh/deployment-operator/pkg/deploymentrequest"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(platform.AddToScheme(scheme))
}

func main() {
	var enableLeaderElection bool

	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "1237ec41.plural.sh",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = (&deploymentrequestpackage.Reconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("DeploymentRequest"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeploymentRequest")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
