package main

import (
	"os"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/controller"
	"github.com/samber/lo"
	"golang.org/x/net/context"
	"k8s.io/client-go/rest"

	"github.com/pluralsh/deployment-operator/pkg/controller/pipelinegates"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
)

func runAgent(opt *options, config *rest.Config, ctx context.Context) (*controller.ControllerManager, *service.ServiceReconciler) {
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

	mgr, err := controller.NewControllerManager(ctx, opt.maxCurrentReconciles, t, r, lo.ToPtr(true), opt.consoleUrl, opt.deployToken, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	sr, err := service.NewServiceReconciler(mgr.GetClient(), config, r, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create service reconciler")
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Gate Controller",
		Do:    sr,
		Queue: sr.SvcQueue,
	})
	gr, err := pipelinegates.NewGateReconciler(mgr.GetClient(), config, r, opt.clusterId)
	if err != nil {
		setupLog.Error(err, "unable to create gate reconciler")
		os.Exit(1)
	}
	mgr.AddController(&controller.Controller{
		Name:  "Gate Controller",
		Do:    gr,
		Queue: gr.GateQueue,
	})

	if err := mgr.Start(); err != nil {
		setupLog.Error(err, "unable to start controller manager")
		os.Exit(1)
	}

	return mgr, sr
}
