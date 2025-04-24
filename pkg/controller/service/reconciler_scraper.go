package service

import (
	"context"
	"strings"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/client"
)

func (s *ServiceReconciler) ScrapeKube(ctx context.Context) {
	logger := log.FromContext(ctx)
	logger.Info("attempting to collect all runtime services for the cluster")
	runtimeServices := map[string]*client.NamespaceVersion{}
	deployments, err := s.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from deployments")
		for _, deployment := range deployments.Items {
			AddRuntimeServiceInfo(deployment.Namespace, deployment.GetLabels(), runtimeServices)
		}
	}

	statefulSets, err := s.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from statefulsets")
		for _, ss := range statefulSets.Items {
			AddRuntimeServiceInfo(ss.Namespace, ss.GetLabels(), runtimeServices)
		}
	}

	hasEBPFDaemonSet := false
	daemonSets, err := s.clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		logger.Info("aggregating from daemonsets")
		for _, ds := range daemonSets.Items {
			AddRuntimeServiceInfo(ds.Namespace, ds.GetLabels(), runtimeServices)

			if cache.IsEBPFDaemonSet(ds) {
				hasEBPFDaemonSet = true
			}
		}
	}

	if err := s.consoleClient.RegisterRuntimeServices(runtimeServices, nil, cache.ServiceMesh(hasEBPFDaemonSet)); err != nil {
		logger.Error(err, "failed to register runtime services, this is an ignorable error but could mean your console needs to be upgraded")
	}
}

func AddRuntimeServiceInfo(namespace string, labels map[string]string, acc map[string]*client.NamespaceVersion) {
	if labels == nil {
		return
	}

	if vsn, ok := labels["app.kubernetes.io/version"]; ok {
		if name, ok := labels["app.kubernetes.io/name"]; ok {
			addVersion(acc, name, vsn)
			acc[name].Namespace = namespace
			if partOf, ok := labels["app.kubernetes.io/part-of"]; ok {
				acc[name].PartOf = partOf
			}
			return
		}

		if name, ok := labels["app.kubernetes.io/part-of"]; ok {
			addVersion(acc, name, vsn)
			acc[name].Namespace = namespace
		}
	}
}

func addVersion(services map[string]*client.NamespaceVersion, name, vsn string) {
	old, ok := services[name]
	if !ok {
		services[name] = &client.NamespaceVersion{
			Version: vsn,
		}
		return
	}

	parsedOld, err := semver.NewVersion(strings.TrimPrefix(old.Version, "v"))
	if err != nil {
		services[name].Version = vsn
		return
	}

	parsedNew, err := semver.NewVersion(strings.TrimPrefix(vsn, "v"))
	if err != nil {
		services[name].Version = vsn
		return
	}

	if parsedNew.LessThan(parsedOld) {
		services[name].Version = vsn
	}
}
