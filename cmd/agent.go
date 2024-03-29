package main

import (
	"os"
	"time"

	"github.com/samber/lo"
	"golang.org/x/net/context"
	"k8s.io/client-go/rest"

	"github.com/pluralsh/deployment-operator/pkg/controller/pipelinegates"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/pluralsh/deployment-operator/pkg/controller/restore"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
)

func runAgent(opt *options, config *rest.Config, ctx context.Context, k8sClient ctrclient.Client) (*controller.ControllerManager, *service.ServiceReconciler, *pipelinegates.GateReconciler) {
	r, err := time.ParseDuration(opt.refreshInterval)
	if err != nil {
		setupLog.Error(err, "unable to get refresh interval")
		os.Exit(1)
	}

	t, err := time.ParseDuration(opt.processingTimeout)
	if err != nil {
		setupLog.Error(err, "unable to get processing timeout")
		os.Exit(1)
	}

	mgr, err := controller.NewControllerManager(ctx, opt.maxConcurrentReconciles, t, r, lo.ToPtr(true), opt.consoleUrl, opt.deployToken, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	sr, err := service.NewServiceReconciler(mgr.GetClient(), config, r, opt.restoreNamespace)
	if err != nil {
		setupLog.Error(err, "unable to create service reconciler")
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Service Controller",
		Do:    sr,
		Queue: sr.SvcQueue,
	})
	gr, err := pipelinegates.NewGateReconciler(mgr.GetClient(), k8sClient, config, r, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create gate reconciler")
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Gate Controller",
		Do:    gr,
		Queue: gr.GateQueue,
	})

	rr := restore.NewRestoreReconciler(mgr.GetClient(), k8sClient, r, opt.restoreNamespace)
	mgr.AddController(&controller.Controller{
		Name:  "Restore Controller",
		Do:    rr,
		Queue: rr.RestoreQueue,
	})

	if err := mgr.Start(); err != nil {
		setupLog.Error(err, "unable to start controller manager")
		os.Exit(1)
	}

	return mgr, sr, gr
}
