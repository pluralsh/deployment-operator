package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/metrics"
)

const (
	// ServiceMeshResourceGroupIstio is a base group name used by Istio
	// Ref: https://github.com/istio/istio/blob/6186a80cb220ecbd7e1cc82044fe3a6fc2876c63/operator/pkg/apis/register.go#L27-L31
	ServiceMeshResourceGroupIstio string = "istio.io"

	// ServiceMeshResourceGroupCilium is a base group name used by Cilium
	// Ref: https://github.com/cilium/cilium/blob/99b4bc0d0b628f22c024f3ea74ef21007a831f52/pkg/k8s/apis/cilium.io/register.go#L7-L8
	ServiceMeshResourceGroupCilium string = "cilium.io"

	// ServiceMeshResourceGroupLinkerd is a base group name used by Linkerd
	// Ref: https://github.com/linkerd/linkerd2/blob/e055c32f31ae7618281fed1eb5c304b0d1389a52/controller/gen/apis/serviceprofile/register.go#L3-L4
	ServiceMeshResourceGroupLinkerd string = "linkerd.io"
)

var (
	// Maps a GroupVersion to resource Kind.
	discoveryCache    cmap.ConcurrentMap[string, bool]
	serviceMesh       = ""
	serviceMeshRWLock = sync.RWMutex{}
)

func init() {
	discoveryCache = cmap.New[bool]()
}

func DiscoveryCache() cmap.ConcurrentMap[string, bool] {
	return discoveryCache
}

func ServiceMesh() *client.ServiceMesh {
	serviceMeshRWLock.RLock()
	defer serviceMeshRWLock.RUnlock()

	switch serviceMesh {
	case ServiceMeshResourceGroupIstio:
		return lo.ToPtr(client.ServiceMeshIstio)
	case ServiceMeshResourceGroupCilium:
		return lo.ToPtr(client.ServiceMeshCilium)
	case ServiceMeshResourceGroupLinkerd:
		return lo.ToPtr(client.ServiceMeshLinkerd)
	default:
		return nil
	}
}

func RunDiscoveryCacheInBackgroundOrDie(ctx context.Context, discoveryClient *discovery.DiscoveryClient) {
	klog.Info("starting discovery cache")
	err := helpers.BackgroundPollUntilContextCancel(ctx, 5*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		if err = updateDiscoveryCache(discoveryClient); err != nil {
			klog.Error(err, "can't fetch API versions")
		}

		metrics.Record().DiscoveryAPICacheRefresh(err)
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start discovery cache in background: %w", err))
	}
}

func updateDiscoveryCache(discoveryClient *discovery.DiscoveryClient) error {
	lists, err := discoveryClient.ServerPreferredResources()

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}

			discoveryCache.Set(fmt.Sprintf("%s/%s", gv.String(), resource.Kind), true)
			discoveryCache.Set(gv.String(), true)

			checkAndUpdateServiceMesh(gv.Group)
		}
	}

	return err
}

func checkAndUpdateServiceMesh(group string) {
	serviceMeshRWLock.Lock()
	defer serviceMeshRWLock.Unlock()

	switch {
	case strings.Contains(group, ServiceMeshResourceGroupIstio):
		serviceMesh = ServiceMeshResourceGroupIstio
	case strings.Contains(group, ServiceMeshResourceGroupCilium):
		serviceMesh = ServiceMeshResourceGroupCilium
	case strings.Contains(group, ServiceMeshResourceGroupLinkerd):
		serviceMesh = ServiceMeshResourceGroupLinkerd
	}
}
