package scraper

import (
	"context"
	"fmt"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
	agentcommon "github.com/pluralsh/deployment-operator/pkg/common"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const name = "Ai Insight Component Scraper"

var (
	AiInsightComponents cmap.ConcurrentMap[Component, bool]
)

func init() {
	AiInsightComponents = cmap.NewStringer[Component, bool]()
}

type Component struct {
	Gvk       schema.GroupVersionKind
	Name      string
	Namespace string
}

func (c Component) String() string {
	return fmt.Sprintf("GVK=%s, Name=%s, Namespace=%s", c.Gvk.String(), c.Name, c.Namespace)
}

func RunAiInsightComponentScraperInBackgroundOrDie(ctx context.Context, k8sClient ctrclient.Client) {
	klog.Info("starting ", name)
	scrapeResources := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	}

	manageByOperatorLabels := map[string]string{
		agentcommon.ManagedByLabel: agentcommon.AgentLabelValue,
	}

	err := helpers.BackgroundPollUntilContextCancel(ctx, 15*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		AiInsightComponents.Clear()

		for _, gvk := range scrapeResources {
			if err := setUnhealthyComponents(ctx, k8sClient, gvk, ctrclient.MatchingLabels(manageByOperatorLabels)); err != nil {
				klog.Error(err, "can't set update component status")
			}
		}

		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start %s in background: %w", name, err))
	}
}

func setUnhealthyComponents(ctx context.Context, k8sClient ctrclient.Client, gvk schema.GroupVersionKind, opts ...ctrclient.ListOption) error {
	// Create an unstructured list with the desired GVK
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	// List resources
	if err := k8sClient.List(ctx, list, opts...); err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	// Iterate over each unstructured object
	for _, item := range list.Items {
		health, err := common.GetResourceHealth(&item)
		if err != nil {
			return err
		}
		if health.Status == common.HealthStatusDegraded {
			AiInsightComponents.Set(Component{
				Gvk:       gvk,
				Name:      item.GetName(),
				Namespace: item.GetNamespace(),
			}, true)
		}
	}
	return nil
}
