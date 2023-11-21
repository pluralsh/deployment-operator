package sync

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (engine *Engine) ScrapeKube() {
	log.Info("attempting to collect all runtime services for the cluster")
	ctx := context.Background()
	runtimeServices := map[string]string{}
	deployments, err := engine.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		log.Info("aggregating from deployments")
		for _, deployment := range deployments.Items {
			AddRuntimeServiceInfo(deployment.GetLabels(), runtimeServices)
		}
	}

	statefulSets, err := engine.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		log.Info("aggregating from statefulsets")
		for _, ss := range statefulSets.Items {
			AddRuntimeServiceInfo(ss.GetLabels(), runtimeServices)
		}
	}

	daemonSets, err := engine.clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		log.Info("aggregating from daemonsets")
		for _, ss := range daemonSets.Items {
			AddRuntimeServiceInfo(ss.GetLabels(), runtimeServices)
		}
	}
	/*	if err := engine.client.RegisterRuntimeServices(runtimeServices, nil); err != nil {
		log.Error(err, "failed to register runtime services, this is an ignorable error but could mean your console needs to be upgraded")
	}*/
}

func AddRuntimeServiceInfo(labels map[string]string, acc map[string]string) {
	if labels == nil {
		return
	}

	if vsn, ok := labels["app.kubernetes.io/version"]; ok {
		if name, ok := labels["app.kubernetes.io/name"]; ok {
			acc[name] = vsn
			return
		}

		if name, ok := labels["app.kubernetes.io/part-of"]; ok {
			acc[name] = vsn
		}
	}
}
