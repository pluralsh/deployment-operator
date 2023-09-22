package main

import (
	"flag"
	"os"

	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	"github.com/pluralsh/deployment-operator/providers/argocd/pkg/status"
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
	utilruntime.Must(application.AddToScheme(scheme))
}

func main() {
	var enableLeaderElection bool
	var debug bool
	var applicationNamespace string
	var deploymentNamespace string

	flag.BoolVar(&debug, "debug", true,
		"Enable debug")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&applicationNamespace, "application-namespace", "argo-cd", "namespace where ArgoCD application will be installed")
	flag.StringVar(&deploymentNamespace, "deployment-namespace", "default", "namespace where Plural deployment will be installed")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "9843ec41.plural.sh",
		Namespace:          applicationNamespace,
		MetricsBindAddress: "localhost:8083",
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = (&status.Reconciler{
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("Application"),
		DeploymentNamespace: deploymentNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Application")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
