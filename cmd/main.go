package main

import (
	"os"

	templatesv1 "github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1"
	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(constraintstatusv1beta1.AddToScheme(scheme))
	utilruntime.Must(templatesv1.AddToScheme(scheme))
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
	ctrlMgr, serviceReconciler, gateReconciler := runAgent(opt, config, ctx, mgr.GetClient())

	backupController := &controller.BackupReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConsoleClient: ctrlMgr.GetClient(),
	}
	restoreController := &controller.RestoreReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConsoleClient: ctrlMgr.GetClient(),
	}
	constraintController := &controller.ConstraintReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		ConsoleClient: ctrlMgr.GetClient(),
		Reader:        mgr.GetCache(),
	}

	reconcileGroups := map[schema.GroupVersionKind]controller.SetupWithManager{
		{
			Group:   velerov1.SchemeGroupVersion.Group,
			Version: velerov1.SchemeGroupVersion.Version,
			Kind:    "Backup",
		}: backupController.SetupWithManager,
		{
			Group:   velerov1.SchemeGroupVersion.Group,
			Version: velerov1.SchemeGroupVersion.Version,
			Kind:    "Restore",
		}: restoreController.SetupWithManager,
		{
			Group:   "status.gatekeeper.sh",
			Version: "v1beta1",
			Kind:    "ConstraintPodStatus",
		}: constraintController.SetupWithManager,
	}

	if err = (&controller.CrdRegisterControllerReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		ReconcilerGroups: reconcileGroups,
		Mgr:              mgr,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CRDRegisterController")
	}

	if err = (&controller.CustomHealthReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ServiceReconciler: serviceReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HealthConvert")
	}
	//+kubebuilder:scaffold:builder

	if err = (&controller.PipelineGateReconciler{
		Client:        mgr.GetClient(),
		GateCache:     gateReconciler.GateCache,
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
