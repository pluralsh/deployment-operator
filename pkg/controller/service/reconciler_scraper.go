package service

import (
	"context"
	"strings"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (s *ServiceReconciler) ScrapeKube(ctx context.Context) {
	logger := log.FromContext(ctx)
	logger.Info("attempting to collect all runtime services for the cluster")
	runtimeServices := map[string]string{}
	deployments, err := s.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from deployments")
		for _, deployment := range deployments.Items {
			AddRuntimeServiceInfo(deployment.GetLabels(), runtimeServices)
		}
	}

	statefulSets, err := s.Clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from statefulsets")
		for _, ss := range statefulSets.Items {
			AddRuntimeServiceInfo(ss.GetLabels(), runtimeServices)
		}
	}

	daemonSets, err := s.Clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from daemonsets")
		for _, ss := range daemonSets.Items {
			AddRuntimeServiceInfo(ss.GetLabels(), runtimeServices)
		}
	}
	if err := s.ConsoleClient.RegisterRuntimeServices(runtimeServices, nil); err != nil {
		logger.Error(err, "failed to register runtime services, this is an ignorable error but could mean your console needs to be upgraded")
	}
}

func AddRuntimeServiceInfo(labels map[string]string, acc map[string]string) {
	if labels == nil {
		return
	}

	if vsn, ok := labels["app.kubernetes.io/version"]; ok {
		if name, ok := labels["app.kubernetes.io/name"]; ok {
			addVersion(acc, name, vsn)
			return
		}

		if name, ok := labels["app.kubernetes.io/part-of"]; ok {
			addVersion(acc, name, vsn)
		}
	}
}

func addVersion(services map[string]string, name, vsn string) {
	old, ok := services[name]
	if !ok {
		services[name] = vsn
		return
	}

	parsedOld, err := semver.NewVersion(strings.TrimPrefix(old, "v"))
	if err != nil {
		services[name] = vsn
		return
	}

	parsedNew, err := semver.NewVersion(strings.TrimPrefix(vsn, "v"))
	if err != nil {
		services[name] = vsn
		return
	}

	if parsedNew.LessThan(parsedOld) {
		services[name] = vsn
	}
}
