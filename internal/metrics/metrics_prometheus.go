package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	recorder = (&prometheusRecorder{}).init()
)

type prometheusRecorder struct {
	discoveryAPICacheRefreshCounter prometheus.Counter
	serviceReconciliationCounter    *prometheus.CounterVec
	stackRunJobsCreatedCounter      prometheus.Counter
}

func (in *prometheusRecorder) DiscoveryAPICacheRefresh() {
	in.discoveryAPICacheRefreshCounter.Inc()
}

func (in *prometheusRecorder) ServiceReconciliation(serviceID, serviceName string) {
	in.serviceReconciliationCounter.WithLabelValues(serviceID, serviceName).Inc()
}

func (in *prometheusRecorder) StackRunJobCreation() {
	in.stackRunJobsCreatedCounter.Inc()
}

func (in *prometheusRecorder) init() Recorder {
	in.discoveryAPICacheRefreshCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: DiscoveryAPICacheRefreshMetricName,
		Help: DiscoveryAPICacheRefreshMetricDescription,
	})

	in.serviceReconciliationCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ServiceReconciliationMetricName,
		Help: ServiceReconciliationMetricDescription,
	}, []string{ServiceReconciliationMetricLabelServiceID, ServiceReconciliationMetricLabelServiceName})

	in.stackRunJobsCreatedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: StackRunJobsCreatedMetricName,
		Help: StackRunJobsCreatedMetricDescription,
	})

	return in
}

func Record() Recorder {
	return recorder
}
