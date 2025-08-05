package ping

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"

	"github.com/Masterminds/semver/v3"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pluralsh/deployment-operator/pkg/common"
	v1 "github.com/pluralsh/deployment-operator/pkg/controller/v1"

	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/client"
)

const runtimeServicePingerName = "runtime service pinger"

func RunRuntimeServicePingerInBackgroundOrDie(ctx context.Context, pinger *Pinger, duration time.Duration) {
	klog.Info("starting ", runtimeServicePingerName)

	err := helpers.BackgroundPollUntilContextCancel(ctx, duration, true, true, func(_ context.Context) (done bool, err error) {
		pinger.PingRuntimeServices(ctx)
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start %s in background: %w", runtimeServicePingerName, err))
	}
}

func (p *Pinger) PingRuntimeServices(ctx context.Context) {
	klog.Info("attempting to collect all runtime services for the cluster")
	// Pre-allocate map with estimated capacity to avoid reallocations
	runtimeServices := make(map[string]client.NamespaceVersion, 100)
	deployments, err := p.clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		klog.Info("aggregating from deployments")
		for _, deployment := range deployments.Items {
			AddRuntimeServiceInfo(deployment.Namespace, deployment.GetLabels(), runtimeServices)
		}
	}

	statefulSets, err := p.clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		klog.Info("aggregating from statefulsets")
		for _, ss := range statefulSets.Items {
			AddRuntimeServiceInfo(ss.Namespace, ss.GetLabels(), runtimeServices)
		}
	}

	hasEBPFDaemonSet := false
	daemonSets, err := p.clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		klog.Info("aggregating from daemonsets")
		for _, ds := range daemonSets.Items {
			AddRuntimeServiceInfo(ds.Namespace, ds.GetLabels(), runtimeServices)

			if cache.IsEBPFDaemonSet(ds) {
				hasEBPFDaemonSet = true
			}
		}
	}

	serviceMesh := cache.ServiceMesh(hasEBPFDaemonSet)
	if serviceMesh == nil {
		klog.Info("no service mesh detected")
	} else {
		klog.Info("detected service mesh", "serviceMesh", serviceMesh)
	}

	if err := p.consoleClient.RegisterRuntimeServices(runtimeServices, p.GetDeprecatedCustomResources(ctx), nil, serviceMesh); err != nil {
		klog.ErrorS(err, "failed to register runtime services, this is an ignorable error but could mean your console needs to be upgraded")
	}

}

func AddRuntimeServiceInfo(namespace string, labels map[string]string, acc map[string]client.NamespaceVersion) {
	if labels == nil {
		return
	}

	vsn, ok := labels["app.kubernetes.io/version"]
	if !ok {
		return
	}

	if name, ok := labels["app.kubernetes.io/name"]; ok {
		acc[name] = addVersion(acc[name], vsn)
		entry := acc[name]
		entry.Namespace = namespace
		if partOf, ok := labels["app.kubernetes.io/part-of"]; ok {
			entry.PartOf = partOf
		}
		acc[name] = entry
		return
	}

	if name, ok := labels["app.kubernetes.io/part-of"]; ok {
		acc[name] = addVersion(acc[name], vsn)
		entry := acc[name]
		entry.Namespace = namespace
		acc[name] = entry
	}
}

func addVersion(entry client.NamespaceVersion, vsn string) client.NamespaceVersion {
	if entry.Version == "" {
		entry.Version = vsn
		return entry
	}

	parsedOld, err := semver.NewVersion(strings.TrimPrefix(entry.Version, "v"))
	if err != nil {
		entry.Version = vsn
		return entry
	}

	parsedNew, err := semver.NewVersion(strings.TrimPrefix(vsn, "v"))
	if err != nil {
		entry.Version = vsn
		return entry
	}

	if parsedNew.LessThan(parsedOld) {
		entry.Version = vsn
	}
	return entry
}

func (p *Pinger) getVersionedCrd(ctx context.Context) (map[string][]v1.NormalizedVersion, error) {
	crdList, err := p.apiExtClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	crdVersionsMap := make(map[string][]v1.NormalizedVersion, len(crdList.Items))
	for _, crd := range crdList.Items {
		kind := crd.Spec.Names.Kind
		group := crd.Spec.Group
		groupKind := fmt.Sprintf("%s/%s", group, kind)
		// Pre-allocate slice with capacity based on the number of versions in the CRD
		parsedVersions := make([]v1.NormalizedVersion, 0, len(crd.Spec.Versions))
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

func (p *Pinger) GetDeprecatedCustomResources(ctx context.Context) []console.DeprecatedCustomResourceAttributes {
	logger := log.FromContext(ctx)
	crds, err := p.getVersionedCrd(ctx)
	if err != nil {
		logger.Error(err, "failed to retrieve versioned CRDs")
		return nil
	}

	// Pre-allocate slice with estimated capacity based on the number of CRDs
	deprecated := make([]console.DeprecatedCustomResourceAttributes, 0, len(crds)*2)
	for groupKind, versions := range crds {
		gkList := strings.Split(groupKind, "/")
		if len(gkList) != 2 {
			continue
		}
		group := gkList[0]
		kind := gkList[1]
		d := p.getDeprecatedCustomResourceObjects(ctx, versions, group, kind)
		deprecated = append(deprecated, d...)
	}
	return deprecated
}

func (p *Pinger) getDeprecatedCustomResourceObjects(ctx context.Context, versions []v1.NormalizedVersion, group, kind string) []console.DeprecatedCustomResourceAttributes {
	// Pre-allocate slice with estimated capacity based on the number of version pairs
	versionPairs := getVersionPairs(versions)
	deprecatedCustomResourceAttributes := make([]console.DeprecatedCustomResourceAttributes, 0, len(versionPairs)*5)
	for _, version := range versionPairs {
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: version.PreviousVersion,
			Kind:    kind,
		}

		pager := common.ListResources(ctx, p.k8sClient, gvk, nil)
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
