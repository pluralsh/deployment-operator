package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const name = "metrics scraper"

var (
	metrics         *Metrics
	nodeCacheExpiry = 30 * time.Minute
)

func init() {
	metrics = &Metrics{
		metrics:     v1alpha1.MetricsAggregateStatus{},
		lastUpdated: time.Now().Add(-nodeCacheExpiry),
	}
}

type Metrics struct {
	mu          sync.RWMutex
	metrics     v1alpha1.MetricsAggregateStatus
	lastUpdated time.Time
}

func GetMetrics() *Metrics {
	return metrics
}

func (s *Metrics) Add(metrics v1alpha1.MetricsAggregateStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metrics = metrics
	s.lastUpdated = time.Now()
}

func (s *Metrics) Get() v1alpha1.MetricsAggregateStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metrics
}

func RunMetricsScraperInBackgroundOrDie(ctx context.Context, k8sClient ctrclient.Client, discoveryClient *discovery.DiscoveryClient, config *rest.Config) {
	klog.Info("starting ", name)

	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("failed to create metrics client: %w", err))
	}

	err = helpers.DynamicBackgroundPollUntilContextCancel(ctx, func() time.Duration { return time.Minute }, true, func(_ context.Context) (done bool, err error) {
		apiGroups, err := discoveryClient.ServerGroups()
		if err == nil {
			metricsAPIAvailable := common.SupportedMetricsAPIVersionAvailable(apiGroups)
			status, err := common.GetMetricsAggregateStatus(ctx, k8sClient, metricsClient, metricsAPIAvailable)
			if err == nil && status != nil {
				GetMetrics().Add(*status)
			} else if err != nil {
				if time.Since(GetMetrics().lastUpdated) > nodeCacheExpiry {
					if cs, err := kubernetes.NewForConfig(config); err == nil {
						getMetricsFromNodes(ctx, cs)
						return false, nil
					}
					klog.ErrorS(err, "failed to get metrics from nodes")
				}
			}
		}
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start %s in background: %w", name, err))
	}
}

func getMetricsFromNodes(ctx context.Context, client *kubernetes.Clientset) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to list nodes")
		return
	}

	status := v1alpha1.MetricsAggregateStatus{}
	for _, node := range nodes.Items {
		status.MemoryTotalBytes += node.Status.Allocatable.Memory().Value()
		status.CPUTotalMillicores += node.Status.Allocatable.Cpu().MilliValue()
		status.MemoryAvailableBytes += node.Status.Capacity.Memory().Value()
		status.CPUAvailableMillicores += node.Status.Capacity.Cpu().MilliValue()
	}
	GetMetrics().Add(status)
}
