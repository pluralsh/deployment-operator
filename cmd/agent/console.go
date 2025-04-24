package main

import (
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/client"
	consolectrl "github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"
	v1 "github.com/pluralsh/deployment-operator/pkg/controller/v1"

	"k8s.io/client-go/rest"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/controller/namespaces"
	"github.com/pluralsh/deployment-operator/pkg/controller/pipelinegates"
	"github.com/pluralsh/deployment-operator/pkg/controller/restore"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
)

func initConsoleManagerOrDie() *consolectrl.Manager {
	mgr, err := consolectrl.NewControllerManager(
		consolectrl.WithMaxConcurrentReconciles(args.MaxConcurrentReconciles()),
		consolectrl.WithCacheSyncTimeout(args.ProcessingTimeout()),
		consolectrl.WithPollInterval(args.PollInterval()),
		consolectrl.WithJitter(args.PollJitter()),
		consolectrl.WithRecoverPanic(true),
		consolectrl.WithConsoleClientArgs(args.ConsoleUrl(), args.DeployToken()),
		consolectrl.WithSocketArgs(args.ClusterId(), args.ConsoleUrl(), args.DeployToken()),
	)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	return mgr
}

const (
	// Use custom (short) poll intervals for these reconcilers.
	pipelineGatesPollInterval = 30 * time.Second
	stacksPollInterval        = 30 * time.Second
)

func registerConsoleReconcilersOrDie(
	mgr *consolectrl.Manager,
	config *rest.Config,
	k8sClient ctrclient.Client,
	scheme *runtime.Scheme,
	consoleClient client.Client,
) {
	mgr.AddReconcilerOrDie(service.Identifier, func() (v1.Reconciler, error) {
		r, err := service.NewServiceReconciler(consoleClient, config, args.ControllerCacheTTL(), args.ManifestCacheTTL(), args.ManifestCacheJitter(), args.RestoreNamespace(), args.ConsoleUrl())
		return r, err
	})

	mgr.AddReconcilerOrDie(pipelinegates.Identifier, func() (v1.Reconciler, error) {
		r, err := pipelinegates.NewGateReconciler(consoleClient, k8sClient, config, pipelineGatesPollInterval)
		return r, err
	})

	mgr.AddReconcilerOrDie(restore.Identifier, func() (v1.Reconciler, error) {
		r := restore.NewRestoreReconciler(consoleClient, k8sClient, args.ControllerCacheTTL(), args.RestoreNamespace())
		return r, nil
	})

	mgr.AddReconcilerOrDie(namespaces.Identifier, func() (v1.Reconciler, error) {
		r := namespaces.NewNamespaceReconciler(consoleClient, k8sClient, args.ControllerCacheTTL())
		return r, nil
	})

	mgr.AddReconcilerOrDie(stacks.Identifier, func() (v1.Reconciler, error) {
		namespace, err := utils.GetOperatorNamespace()
		if err != nil {
			setupLog.Error(err, "unable to get operator namespace")
			os.Exit(1)
		}

		r := stacks.NewStackReconciler(consoleClient, k8sClient, scheme, args.ControllerCacheTTL(), stacksPollInterval, namespace, args.ConsoleUrl(), args.DeployToken())
		return r, nil
	})
}
