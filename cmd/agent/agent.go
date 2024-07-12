package main

import (
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/controller/stacks"

	"github.com/samber/lo"
	"golang.org/x/net/context"
	"k8s.io/client-go/rest"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/controller/namespaces"
	"github.com/pluralsh/deployment-operator/pkg/controller/pipelinegates"
	"github.com/pluralsh/deployment-operator/pkg/controller/restore"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
)

const pollInterval = time.Second * 30

func runAgent(config *rest.Config, ctx context.Context, k8sClient ctrclient.Client) (*controller.ControllerManager, *service.ServiceReconciler, *pipelinegates.GateReconciler) {
	mgr, err := controller.NewControllerManager(
		ctx,
		args.MaxConcurrentReconciles(),
		args.ProcessingTimeout(),
		args.RefreshInterval(),
		lo.ToPtr(true),
		args.ConsoleUrl(),
		args.DeployToken(),
		args.ClusterId(),
	)
	if err != nil {
		setupLog.Errorw("unable to create manager", "error", err)
		os.Exit(1)
	}

	sr, err := service.NewServiceReconciler(ctx, mgr.GetClient(), config, args.RefreshInterval(), args.ManifestCacheTTL(), args.RestoreNamespace(), args.ConsoleUrl())
	if err != nil {
		setupLog.Errorw("unable to create service reconciler", "error", err)
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Service Controller",
		Do:    sr,
		Queue: sr.SvcQueue,
	})
	gr, err := pipelinegates.NewGateReconciler(mgr.GetClient(), k8sClient, config, args.RefreshInterval(), pollInterval, args.ClusterId())
	if err != nil {
		setupLog.Errorw("unable to create gate reconciler", "error", err)
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Gate Controller",
		Do:    gr,
		Queue: gr.GateQueue,
	})

	rr := restore.NewRestoreReconciler(mgr.GetClient(), k8sClient, args.RefreshInterval(), args.RestoreNamespace())
	mgr.AddController(&controller.Controller{
		Name:  "Restore Controller",
		Do:    rr,
		Queue: rr.RestoreQueue,
	})

	ns := namespaces.NewNamespaceReconciler(mgr.GetClient(), k8sClient, args.RefreshInterval())
	mgr.AddController(&controller.Controller{
		Name:  "Managed Namespace Controller",
		Do:    ns,
		Queue: ns.NamespaceQueue,
	})

	namespace, err := utils.GetOperatorNamespace()
	if err != nil {
		setupLog.Errorw("unable to get operator namespace", "error", err)
		os.Exit(1)
	}

	s := stacks.NewStackReconciler(mgr.GetClient(), k8sClient, args.RefreshInterval(), pollInterval, namespace, args.ConsoleUrl(), args.DeployToken())
	mgr.AddController(&controller.Controller{
		Name:  "Stack Controller",
		Do:    s,
		Queue: s.StackQueue,
	})
	if err := mgr.Start(); err != nil {
		setupLog.Errorw("unable to start controller manager", "error", err)
		os.Exit(1)
	}

	return mgr, sr, gr
}
