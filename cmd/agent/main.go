package main

import (
	"context"
	"os"
	"time"

	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	templatesv1 "github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1"
	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/client"
	consolectrl "github.com/pluralsh/deployment-operator/pkg/controller"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = klog.NewKlogr()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deploymentsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(constraintstatusv1beta1.AddToScheme(scheme))
	utilruntime.Must(templatesv1.AddToScheme(scheme))
	utilruntime.Must(rolloutv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

const (
	httpClientTimout = time.Second * 5
)

func main() {
	args.Init()
	config := ctrl.GetConfigOrDie()
	ctx := ctrl.LoggerInto(ctrl.SetupSignalHandler(), setupLog)

	extConsoleClient := client.New(args.ConsoleUrl(), args.DeployToken())
	discoveryClient := initDiscoveryClientOrDie(config)
	kubeManager := initKubeManagerOrDie(config)
	consoleManager := initConsoleManagerOrDie()

	// Initialize Pipeline Gate Cache
	cache.InitGateCache(args.ControllerCacheTTL(), extConsoleClient)

	registerConsoleReconcilersOrDie(consoleManager, config, kubeManager.GetClient(), extConsoleClient)
	registerKubeReconcilersOrDie(kubeManager, consoleManager, config, extConsoleClient)

	//+kubebuilder:scaffold:builder

	// Start resource cache in background if enabled.
	if args.ResourceCacheEnabled() {
		cache.Init(ctx, config, args.ResourceCacheTTL())
	}

	// Start the discovery cache in background.
	cache.RunDiscoveryCacheInBackgroundOrDie(ctx, discoveryClient)

	// Start the console manager in background.
	runConsoleManagerInBackgroundOrDie(ctx, consoleManager)

	// Start the standard kubernetes manager and block the main thread until context cancel.
	runKubeManagerOrDie(ctx, kubeManager)
}

func initDiscoveryClientOrDie(config *rest.Config) *discovery.DiscoveryClient {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	return discoveryClient
}

func runConsoleManagerInBackgroundOrDie(ctx context.Context, mgr *consolectrl.Manager) {
	setupLog.Info("starting console controller manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "unable to start console controller manager")
		os.Exit(1)
	}
}

func runKubeManagerOrDie(ctx context.Context, mgr ctrl.Manager) {
	setupLog.Info("starting kubernetes controller manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "unable to start kubernetes controller manager")
		os.Exit(1)
	}
}
