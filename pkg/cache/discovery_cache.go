package cache

//
// import (
//	"context"
//	"fmt"
//	"strings"
//	"sync"
//	"time"
//
//	cmap "github.com/orcaman/concurrent-map/v2"
//	"github.com/pluralsh/console/go/client"
//	"github.com/samber/lo"
//	appsv1 "k8s.io/api/apps/v1"
//	"k8s.io/apimachinery/pkg/runtime/schema"
//	"k8s.io/client-go/discovery"
//	"k8s.io/klog/v2"
//
//	"github.com/pluralsh/deployment-operator/internal/helpers"
//	"github.com/pluralsh/deployment-operator/internal/metrics"
//	"github.com/pluralsh/deployment-operator/pkg/log"
//)
//
//const (
//	ServiceMeshResourcePriorityIstio ServiceMeshResourcePriority = iota
//	ServiceMeshResourcePriorityCilium
//	ServiceMeshResourcePriorityLinkerd
//	ServiceMeshResourcePriorityNone = 255
//
//	// ServiceMeshResourceGroupIstio is a base group name used by Istio
//	// Ref: https://github.com/istio/istio/blob/6186a80cb220ecbd7e1cc82044fe3a6fc2876c63/operator/pkg/apis/register.go#L27-L31
//	ServiceMeshResourceGroupIstio ServiceMeshResourceGroup = "istio.io"
//
//	// ServiceMeshResourceGroupCilium is a base group name used by Cilium
//	// Ref: https://github.com/cilium/cilium/blob/99b4bc0d0b628f22c024f3ea74ef21007a831f52/pkg/k8s/apis/cilium.io/register.go#L7-L8
//	ServiceMeshResourceGroupCilium ServiceMeshResourceGroup = "cilium.io"
//
//	// ServiceMeshResourceGroupLinkerd is a base group name used by Linkerd
//	// Ref: https://github.com/linkerd/linkerd2/blob/e055c32f31ae7618281fed1eb5c304b0d1389a52/controller/gen/apis/serviceprofile/register.go#L3-L4
//	ServiceMeshResourceGroupLinkerd ServiceMeshResourceGroup = "linkerd.io"
//
//	// ServiceMeshResourceGroupNone represents an empty or unknown service mesh
//	ServiceMeshResourceGroupNone ServiceMeshResourceGroup = ""
//
//	appNameLabel = "app.kubernetes.io/name"
//	ebpfAppName  = "opentelemetry-ebpf"
//)
//
//var (
//	// Maps a GroupVersion to resource Kind.
//	discoveryCacheInstance discoveryCache
//	serviceMesh            = ServiceMeshResourceGroupNone
//	serviceMeshRWLock      = sync.RWMutex{}
//)
//
//func init() {
//	discoveryCacheInstance = discoveryCache{apiVersions: cmap.New[bool]()}
//}
//
//type discoveryCache struct {
//	apiVersions cmap.ConcurrentMap[string, bool]
//}
//
//func DiscoveryCache() discoveryCache {
//	return discoveryCacheInstance
//}
//
//// ServiceMeshResourcePriority determines the order in which the ServiceMeshResourceGroup
//// is assigned. Lower number means higher priority.
//type ServiceMeshResourcePriority uint8
//
//// ServiceMeshResourceGroup represents a group name used by a service mesh.
//type ServiceMeshResourceGroup string
//
//func (in ServiceMeshResourceGroup) String() string {
//	return string(in)
//}
//
//func (in ServiceMeshResourceGroup) Priority() ServiceMeshResourcePriority {
//	switch in {
//	case ServiceMeshResourceGroupIstio:
//		return ServiceMeshResourcePriorityIstio
//	case ServiceMeshResourceGroupCilium:
//		return ServiceMeshResourcePriorityCilium
//	case ServiceMeshResourceGroupLinkerd:
//		return ServiceMeshResourcePriorityLinkerd
//	default:
//		return ServiceMeshResourcePriorityNone
//	}
//}
//
//func RunDiscoveryCacheInBackgroundOrDie(ctx context.Context, discoveryClient *discovery.DiscoveryClient) {
//	klog.V(log.LogLevelMinimal).Info("starting discovery cache")
//
//	err := helpers.BackgroundPollUntilContextCancel(ctx, 5*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
//		if err = updateDiscoveryCache(discoveryClient); err != nil {
//			klog.Error(err, "can't fetch API versions")
//		}
//
//		metrics.Record().DiscoveryAPICacheRefresh(err)
//		return false, nil
//	})
//
//	if err != nil {
//		panic(fmt.Errorf("failed to start discovery cache in background: %w", err))
//	}
//}
//
//func ServiceMesh(hasEBPFDaemonSet bool) *client.ServiceMesh {
//	if hasEBPFDaemonSet {
//		return lo.ToPtr(client.ServiceMeshEbpf)
//	}
//
//	serviceMeshRWLock.RLock()
//	defer serviceMeshRWLock.RUnlock()
//
//	switch serviceMesh {
//	case ServiceMeshResourceGroupIstio:
//		return lo.ToPtr(client.ServiceMeshIstio)
//	case ServiceMeshResourceGroupCilium:
//		return lo.ToPtr(client.ServiceMeshCilium)
//	case ServiceMeshResourceGroupLinkerd:
//		return lo.ToPtr(client.ServiceMeshLinkerd)
//	default:
//		return nil
//	}
//}
//
//func IsEBPFDaemonSet(ds appsv1.DaemonSet) bool {
//	value, ok := ds.Labels[appNameLabel]
//	return ok && value == ebpfAppName
//}
//
//func updateDiscoveryCache(discoveryClient *discovery.DiscoveryClient) error {
//	lists, err := discoveryClient.ServerPreferredResources()
//
//	for _, list := range lists {
//		if len(list.APIResources) == 0 {
//			continue
//		}
//
//		gv, err := schema.ParseGroupVersion(list.GroupVersion)
//		if err != nil {
//			continue
//		}
//
//		for _, resource := range list.APIResources {
//			if len(resource.Verbs) == 0 {
//				continue
//			}
//
//			discoveryCacheInstance.apiVersions.Set(fmt.Sprintf("%s/%s", gv.String(), resource.Kind), true)
//			discoveryCacheInstance.apiVersions.Set(gv.String(), true)
//
//			checkAndUpdateServiceMesh(gv.Group)
//		}
//	}
//
//	return err
//}
//
//func checkAndUpdateServiceMesh(group string) {
//	serviceMeshRWLock.Lock()
//	defer serviceMeshRWLock.Unlock()
//
//	newServiceMesh := ServiceMeshResourceGroupNone
//
//	switch {
//	case strings.Contains(group, ServiceMeshResourceGroupIstio.String()):
//		newServiceMesh = ServiceMeshResourceGroupIstio
//	case strings.Contains(group, ServiceMeshResourceGroupCilium.String()):
//		newServiceMesh = ServiceMeshResourceGroupCilium
//	case strings.Contains(group, ServiceMeshResourceGroupLinkerd.String()):
//		newServiceMesh = ServiceMeshResourceGroupLinkerd
//	}
//
//	// Lower number means higher priority, so override only if
//	// new resource group name matches service mesh with lower
//	// priority number.
//	if serviceMesh.Priority() > newServiceMesh.Priority() {
//		serviceMesh = newServiceMesh
//	}
//}
