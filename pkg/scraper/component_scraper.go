package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	controllercommon "github.com/pluralsh/deployment-operator/pkg/controller/common"

	"github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	"github.com/pluralsh/polly/algorithms"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	agentcommon "github.com/pluralsh/deployment-operator/pkg/common"
)

const (
	name     = "Ai Insight Component Scraper"
	nodeKind = "Node"
)

var (
	aiInsightComponents *AiInsightComponents
)

func init() {
	aiInsightComponents = &AiInsightComponents{
		items: containers.NewSet[Component](),
	}
}

type AiInsightComponents struct {
	mu    sync.RWMutex
	items containers.Set[Component]
	fresh bool
}

type Component struct {
	Gvk       schema.GroupVersionKind
	Name      string
	Namespace string
}

func (c Component) String() string {
	return fmt.Sprintf("GVK=%s, Name=%s, Namespace=%s", c.Gvk.String(), c.Name, c.Namespace)
}

func GetAiInsightComponents() *AiInsightComponents {
	return aiInsightComponents
}

func (s *AiInsightComponents) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = containers.NewSet[Component]()
}

func (s *AiInsightComponents) AddItem(c Component) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items.Add(c)
}

func (s *AiInsightComponents) GetItems() []Component {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items.List()
}

func (s *AiInsightComponents) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items.Len()
}

func (s *AiInsightComponents) IsFresh() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fresh
}

func (s *AiInsightComponents) SetFresh(f bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fresh = f
}

func RunAiInsightComponentScraperInBackgroundOrDie(ctx context.Context, k8sClient ctrclient.Client, discoveryClient *discovery.DiscoveryClient) {
	klog.Info("starting ", name)
	scrapeResources := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
		{Group: "", Version: "v1", Kind: nodeKind},
	}
	certificateGVK := schema.GroupVersionKind{Group: certmanager.GroupName, Version: "v1", Kind: "Certificate"}
	scrapeResourcesSet := containers.ToSet[schema.GroupVersionKind](scrapeResources)

	err := helpers.BackgroundPollUntilContextCancel(ctx, 15*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		GetAiInsightComponents().Clear()
		apiGroups, err := discoveryClient.ServerGroups()
		if err == nil {
			if SupportedCertificateAPIVersionAvailable(apiGroups) {
				scrapeResourcesSet.Add(certificateGVK)
			}
		} else {
			klog.Error(err, "can't get the supported groups")
		}

		for _, gvk := range scrapeResourcesSet.List() {
			if err := setUnhealthyComponents(ctx, k8sClient, gvk); err != nil {
				klog.Error(err, "can't set update component status")
			}
		}
		GetAiInsightComponents().SetFresh(true)

		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start %s in background: %w", name, err))
	}
}

func setUnhealthyComponents(ctx context.Context, k8sClient ctrclient.Client, gvk schema.GroupVersionKind) error {
	pager := listResources(ctx, k8sClient, gvk)
	for pager.HasNext() {
		items, err := pager.NextPage()
		if err != nil {
			return err
		}
		for _, item := range items {
			health, err := agentcommon.GetResourceHealth(&item)
			if err != nil {
				return err
			}
			if health.Status == agentcommon.HealthStatusDegraded {
				GetAiInsightComponents().AddItem(Component{
					Gvk:       gvk,
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
				})
			}
		}
	}
	return nil
}

func listResources(ctx context.Context, k8sClient ctrclient.Client, gvk schema.GroupVersionKind) *algorithms.Pager[unstructured.Unstructured] {
	var opts []ctrclient.ListOption
	manageByOperatorLabels := map[string]string{
		agentcommon.ManagedByLabel: agentcommon.AgentLabelValue,
	}
	ml := ctrclient.MatchingLabels(manageByOperatorLabels)
	if gvk != corev1.SchemeGroupVersion.WithKind(nodeKind) {
		opts = append(opts, ml)
	}

	fetch := func(page *string, size int64) ([]unstructured.Unstructured, *algorithms.PageInfo, error) {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)

		if page != nil {
			opts = append(opts, ctrclient.Continue(*page))
		}
		opts = append(opts, ctrclient.Limit(size))
		// List resources
		if err := k8sClient.List(ctx, list, opts...); err != nil {
			return nil, nil, fmt.Errorf("failed to list resources: %w", err)
		}
		pageInfo := &algorithms.PageInfo{
			HasNext:  list.GetContinue() != "",
			After:    lo.ToPtr(list.GetContinue()),
			PageSize: size,
		}
		return list.Items, pageInfo, nil
	}
	return algorithms.NewPager[unstructured.Unstructured](controllercommon.DefaultPageSize, fetch)
}

func SupportedCertificateAPIVersionAvailable(discoveredAPIGroups *metav1.APIGroupList) bool {
	for _, discoveredAPIGroup := range discoveredAPIGroups.Groups {
		if discoveredAPIGroup.Name != certmanager.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			for _, supportedVersion := range []string{"v1"} {
				if version.Version == supportedVersion {
					return true
				}
			}
		}
	}
	return false
}
