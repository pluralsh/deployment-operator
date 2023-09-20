package main

import (
	"context"
	"flag"
	"os"

	"github.com/pluralsh/deployment-operator/pkg/deployment"
	"github.com/pluralsh/deployment-operator/provisioner"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"

	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
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
	var debug bool
	var driverAddress string

	flag.BoolVar(&debug, "debug", true,
		"Enable debug")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&driverAddress, "driver-addr", "unix:///var/lib/deployment/deployment.sock", "path to unix domain socket where driver is listening")
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

	ctxInfo := context.Background()
	provisionerClient, err := provisioner.NewDefaultProvisionerClient(ctxInfo, driverAddress, debug)
	if err != nil {
		setupLog.Error(err, "unable to create provisioner client")
		os.Exit(1)
	}
	info, err := provisionerClient.DriverGetInfo(ctxInfo, &proto.DriverGetInfoRequest{})
	if err != nil {
		setupLog.Error(err, "unable to get driver info")
		os.Exit(1)
	}
	if err = (&deployment.Reconciler{
		Client:            mgr.GetClient(),
		Log:               ctrl.Log.WithName("controllers").WithName("Deployment"),
		DriverName:        info.Name,
		ProvisionerClient: provisionerClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Deployment")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}

}
