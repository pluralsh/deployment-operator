package main

import (
	"context"
	"os"
	"time"

	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"

	"github.com/pluralsh/deployment-operator/pkg/ping"

	kubernetestrace "github.com/DataDog/dd-trace-go/contrib/k8s.io/client-go/v2/kubernetes"
	datadogtracer "github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	datadogprofiler "github.com/DataDog/dd-trace-go/v2/profiler"
	trivy "github.com/aquasecurity/trivy-operator/pkg/apis/aquasecurity/v1alpha1"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	templatesv1 "github.com/open-policy-agent/frameworks/constraint/pkg/apis/templates/v1"
	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	openshift "github.com/openshift/api/config/v1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	deploymentsv1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/client"
	consolectrl "github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/scraper"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = klog.NewKlogr()
)

func init() {
	utilruntime.Must(trivy.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(deploymentsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(velerov1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(constraintstatusv1beta1.AddToScheme(scheme))
	utilruntime.Must(templatesv1.AddToScheme(scheme))
	utilruntime.Must(rolloutv1alpha1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(openshift.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

const (
	httpClientTimout = time.Second * 5
)

func main() {
	args.Init()
	config := ctrl.GetConfigOrDie()
	ctx := ctrl.LoggerInto(ctrl.SetupSignalHandler(), setupLog)

	if args.PyroscopeEnabled() {
		profiler, err := args.InitPyroscope()
		if err != nil {
			setupLog.Error(err, "unable to initialize pyroscope")
			os.Exit(1)
		}

		defer func() {
			_ = profiler.Stop()
		}()
	}

	if args.DatadogEnabled() {
		err := args.InitDatadog()
		if err != nil {
			panic("unable to initialize datadog")
		}

		// Trace kubernetes client calls
		config.WrapTransport = kubernetestrace.WrapRoundTripper

		defer func() {
			datadogtracer.Stop()
			datadogprofiler.Stop()
		}()
	}

	extConsoleClient := client.New(args.ConsoleUrl(), args.DeployToken())
	discoveryClient := initDiscoveryClientOrDie(config)
	dynamicClient := initDynamicClientOrDie(config)
	kubeManager := initKubeManagerOrDie(config)
	consoleManager := initConsoleManagerOrDie()
	pinger := ping.NewOrDie(extConsoleClient, config, kubeManager.GetClient())

	// Initialize Pipeline Gate Cache
	cache.InitGateCache(args.ControllerCacheTTL(), extConsoleClient)

	dbStore, err := store.NewDatabaseStore(store.WithStorage(store.StorageFile), store.WithFilePath("/tmp/agent-store.db"))
	if err != nil {
		setupLog.Error(err, "unable to initialize database store")
		os.Exit(1)
	}
	defer dbStore.Shutdown()

	streamline.InitGlobalStore(dbStore)

	// Start synchronizer supervisor
	streamline.Run(dynamicClient, dbStore)

	registerConsoleReconcilersOrDie(consoleManager, config, kubeManager.GetClient(), dynamicClient, dbStore, kubeManager.GetScheme(), extConsoleClient)
	registerKubeReconcilersOrDie(ctx, kubeManager, consoleManager, config, extConsoleClient, discoveryClient, args.EnableKubecostProxy())

	//+kubebuilder:scaffold:builder

	// Start resource cache in background if enabled.
	//if args.ResourceCacheEnabled() {
	//	db.Init()
	//	cache.Init(ctx, config, args.ResourceCacheTTL())
	//	scraper.RunServerGroupsScraperInBackgroundOrDie(ctx, config)
	//}

	// Start the discovery cache in background.
	cache.RunDiscoveryCacheInBackgroundOrDie(ctx, discoveryClient)

	// Start the metrics scarper in background.
	scraper.RunMetricsScraperInBackgroundOrDie(ctx, kubeManager.GetClient(), discoveryClient, config)

	// Start the console manager in background.
	runConsoleManagerInBackgroundOrDie(ctx, consoleManager)

	// Start cluster pinger
	ping.RunClusterPingerInBackgroundOrDie(ctx, pinger, args.ClusterPingInterval())

	// Start runtime services pinger
	ping.RunRuntimeServicePingerInBackgroundOrDie(ctx, pinger, args.RuntimeServicesPingInterval())

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

func initDynamicClientOrDie(config *rest.Config) dynamic.Interface {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create dynamic client")
		os.Exit(1)
	}
	return dynamicClient
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
