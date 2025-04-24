package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	console "github.com/pluralsh/console/go/client"
	v1 "github.com/pluralsh/deployment-operator/pkg/controller/v1"
	"github.com/pluralsh/deployment-operator/pkg/scraper"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	serviceMesh := cache.ServiceMesh(hasEBPFDaemonSet)
	logger.Info("detected service mesh", "serviceMesh", serviceMesh)
	if err := s.consoleClient.RegisterRuntimeServices(runtimeServices, s.GetDeprecatedCustomResources(ctx), nil, serviceMesh); err != nil {
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

func (s *ServiceReconciler) getVersionedCrd(ctx context.Context) (map[string][]v1.NormalizedVersion, error) {
	crdList, err := s.apiExtClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	crdVersionsMap := make(map[string][]v1.NormalizedVersion, len(crdList.Items))
	for _, crd := range crdList.Items {
		kind := crd.Spec.Names.Kind
		group := crd.Spec.Group
		groupKind := fmt.Sprintf("%s/%s", group, kind)
		var parsedVersions []v1.NormalizedVersion
		for _, v := range crd.Spec.Versions {
			parsed, ok := v1.ParseVersion(v.Name)
			if !ok {
				continue
			}
			// flag enabling/disabling this version from being served via REST APIs
			if !v.Served {
				continue
			}
			parsedVersions = append(parsedVersions, *parsed)
		}
		sort.Slice(parsedVersions, func(i, j int) bool {
			return v1.CompareVersions(parsedVersions[i], parsedVersions[j])
		})
		crdVersionsMap[groupKind] = parsedVersions
	}

	return crdVersionsMap, nil
}

func (s *ServiceReconciler) GetDeprecatedCustomResources(ctx context.Context) []console.DeprecatedCustomResourceAttributes {
	logger := log.FromContext(ctx)
	crds, err := s.getVersionedCrd(ctx)
	if err != nil {
		logger.Error(err, "failed to retrieve versioned CRDs")
		return nil
	}

	var deprecated []console.DeprecatedCustomResourceAttributes
	for groupKind, versions := range crds {
		gkList := strings.Split(groupKind, "/")
		if len(gkList) != 2 {
			continue
		}
		group := gkList[0]
		kind := gkList[1]
		d := s.getDeprecatedCustomResourceObjects(ctx, versions, group, kind)
		deprecated = append(deprecated, d...)
	}
	return deprecated
}

func (s *ServiceReconciler) getDeprecatedCustomResourceObjects(ctx context.Context, versions []v1.NormalizedVersion, group, kind string) []console.DeprecatedCustomResourceAttributes {
	var deprecatedCustomResourceAttributes []console.DeprecatedCustomResourceAttributes
	versionPairs := getVersionPairs(versions)
	for _, version := range versionPairs {
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: version.PreviousVersion,
			Kind:    kind,
		}

		pager := scraper.ListResources(ctx, s.k8sClient, gvk, nil)
		for pager.HasNext() {
			items, err := pager.NextPage()
			if err != nil {
				break
			}
			for _, item := range items {
				attr := console.DeprecatedCustomResourceAttributes{
					Group:       group,
					Kind:        kind,
					Name:        item.GetName(),
					Version:     version.PreviousVersion,
					NextVersion: version.LatestVersion,
				}
				if item.GetNamespace() != "" {
					attr.Namespace = lo.ToPtr(item.GetNamespace())
				}
				deprecatedCustomResourceAttributes = append(deprecatedCustomResourceAttributes, attr)
			}
		}
	}
	return deprecatedCustomResourceAttributes
}

type VersionPair struct {
	LatestVersion   string
	PreviousVersion string
}

func getVersionPairs(versions []v1.NormalizedVersion) []VersionPair {
	// Helper function for creating VersionPair
	createVersionPair := func(latest, previous v1.NormalizedVersion) VersionPair {
		return VersionPair{
			LatestVersion:   latest.Raw,
			PreviousVersion: previous.Raw,
		}
	}

	versionPairs := make([]VersionPair, 0, len(versions)-1) // Preallocate slice capacity
	for i := 0; i < len(versions)-1; i++ {
		versionPair := createVersionPair(versions[i], versions[i+1])
		versionPairs = append(versionPairs, versionPair)
	}
	return versionPairs
}
