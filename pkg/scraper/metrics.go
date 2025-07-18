package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const name = "metrics scraper"

var (
	metrics *Metrics
)

func init() {
	metrics = &Metrics{
		metrics: v1alpha1.MetricsAggregateStatus{},
	}
}

type Metrics struct {
	mu      sync.RWMutex
	metrics v1alpha1.MetricsAggregateStatus
}

func GetMetrics() *Metrics {
	return metrics
}

func (s *Metrics) Add(metrics v1alpha1.MetricsAggregateStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = metrics
}

func (s *Metrics) Get() v1alpha1.MetricsAggregateStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func RunMetricsScraperInBackgroundOrDie(ctx context.Context, k8sClient ctrclient.Client, discoveryClient *discovery.DiscoveryClient, metricsClient metricsclientset.Interface) {
	klog.Info("starting ", name)

	err := helpers.BackgroundPollUntilContextCancel(ctx, time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		apiGroups, err := discoveryClient.ServerGroups()
		if err == nil {
			metricsAPIAvailable := common.SupportedMetricsAPIVersionAvailable(apiGroups)
			if metricsAPIAvailable {
				status, err := common.GetMetricsAggregateStatus(ctx, k8sClient, metricsClient)
				if err == nil && status != nil {
					GetMetrics().Add(*status)
				} else if err != nil {
					klog.Error(err, "failed to get metrics")
				}
			}
		}
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start %s in background: %w", name, err))
	}
}
