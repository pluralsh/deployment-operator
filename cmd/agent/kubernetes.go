package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	roclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	consolectrl "github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func initKubeManagerOrDie(config *rest.Config) manager.Manager {
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Logger:                 setupLog,
		Scheme:                 scheme,
		LeaderElection:         args.EnableLeaderElection(),
		LeaderElectionID:       "dep12loy45.plural.sh",
		HealthProbeBindAddress: args.ProbeAddr(),
		Metrics: server.Options{
			BindAddress: args.MetricsAddr(),
			ExtraHandlers: map[string]http.Handler{
				// Default prometheus metrics path.
				// We can't use /metrics as it is already taken by the
				// controller manager.
				"/metrics/agent": promhttp.Handler(),
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err = mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	return mgr
}

func initKubeClientsOrDie(config *rest.Config) (rolloutsClient *roclientset.Clientset, dynamicClient *dynamic.DynamicClient, kubeClient *kubernetes.Clientset, metricsClient metricsclientset.Interface) {
	rolloutsClient, err := roclientset.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create rollouts client")
		os.Exit(1)
	}

	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}

	kubeClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create kubernetes client")
		os.Exit(1)
	}

	metricsClient, err = metricsclientset.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create metrics client")
		os.Exit(1)
	}

	return rolloutsClient, dynamicClient, kubeClient, metricsClient
}

func registerKubeReconcilersOrDie(
	ctx context.Context,
	manager ctrl.Manager,
	consoleManager *consolectrl.Manager,
	config *rest.Config,
	extConsoleClient consoleclient.Client,
	discoveryClient discovery.DiscoveryInterface,
) {

	rolloutsClient, dynamicClient, kubeClient, metricsClient := initKubeClientsOrDie(config)

	backupController := &controller.BackupReconciler{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
	}
	restoreController := &controller.RestoreReconciler{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
	}
	constraintController := &controller.ConstraintReconciler{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
		Reader:        manager.GetCache(),
	}
	argoRolloutController := &controller.ArgoRolloutReconciler{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
		ConsoleURL:    args.ConsoleUrl(),
		HttpClient:    &http.Client{Timeout: httpClientTimout},
		ArgoClientSet: rolloutsClient,
		DynamicClient: dynamicClient,
		SvcReconciler: consoleManager.GetReconcilerOrDie(service.Identifier),
		KubeClient:    kubeClient,
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
		{
			Group:   rolloutv1alpha1.SchemeGroupVersion.Group,
			Version: rolloutv1alpha1.SchemeGroupVersion.Version,
			Kind:    rollouts.RolloutKind,
		}: argoRolloutController.SetupWithManager,
	}

	if err := (&controller.CrdRegisterControllerReconciler{
		Client:           manager.GetClient(),
		Scheme:           manager.GetScheme(),
		ReconcilerGroups: reconcileGroups,
		Mgr:              manager,
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CRDRegisterController")
	}

	if err := (&controller.CustomHealthReconciler{
		Client: manager.GetClient(),
		Scheme: manager.GetScheme(),
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HealthConvert")
	}
	if err := (&controller.StackRunJobReconciler{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StackRun")
	}

	if err := (&controller.IngressReplicaReconciler{
		Client: manager.GetClient(),
		Scheme: manager.GetScheme(),
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IngressReplica")
	}

	rawConsoleUrl, _ := strings.CutSuffix(args.ConsoleUrl(), "/ext/gql")
	if err := (&controller.VirtualClusterController{
		Client:           manager.GetClient(),
		Scheme:           manager.GetScheme(),
		ExtConsoleClient: extConsoleClient,
		ConsoleUrl:       rawConsoleUrl,
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "VirtualCluster")
	}

	if err := (&controller.UpgradeInsightsController{
		Client:        manager.GetClient(),
		Scheme:        manager.GetScheme(),
		ConsoleClient: extConsoleClient,
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "UpgradeInsights")
	}

	statusController, err := controller.NewStatusReconciler(manager.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StatusController")
	}
	if err := statusController.SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to setup controller", "controller", "StatusController")
	}

	if err = (&controller.PipelineGateReconciler{
		Client:        manager.GetClient(),
		ConsoleClient: consoleclient.New(args.ConsoleUrl(), args.DeployToken()),
		Scheme:        manager.GetScheme(),
		GateCache:     cache.GateCache(),
	}).SetupWithManager(manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Group")
		os.Exit(1)
	}

	if err := (&controller.MetricsAggregateReconciler{
		Client:          manager.GetClient(),
		Scheme:          manager.GetScheme(),
		DiscoveryClient: discoveryClient,
		MetricsClient:   metricsClient,
	}).SetupWithManager(ctx, manager); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MetricsAggregate")
	}
}
