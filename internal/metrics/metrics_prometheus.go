package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	recorder = (&prometheusRecorder{}).init()
)

type prometheusRecorder struct {
	discoveryAPICacheRefreshCounter      prometheus.Counter
	discoveryAPICacheRefreshErrorCounter prometheus.Counter
	serviceReconciliationCounter         *prometheus.CounterVec
	serviceReconciliationErrorCounter    *prometheus.CounterVec
	stackRunJobsCreatedCounter           prometheus.Counter
}

func (in *prometheusRecorder) DiscoveryAPICacheRefresh(err error) {
	if err != nil {
		in.discoveryAPICacheRefreshErrorCounter.Inc()
		return
	}

	in.discoveryAPICacheRefreshCounter.Inc()
}

func (in *prometheusRecorder) ServiceReconciliation(serviceID, serviceName string, err error) {
	if err != nil {
		in.serviceReconciliationErrorCounter.WithLabelValues(serviceID, serviceName).Inc()
		return
	}

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

	in.discoveryAPICacheRefreshErrorCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: DiscoveryAPICacheRefreshErrorMetricName,
		Help: DiscoveryAPICacheRefreshErrorMetricDescription,
	})

	in.serviceReconciliationCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ServiceReconciliationMetricName,
		Help: ServiceReconciliationMetricDescription,
	}, []string{ServiceReconciliationMetricLabelServiceID, ServiceReconciliationMetricLabelServiceName})

	in.serviceReconciliationErrorCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: ServiceReconciliationErrorMetricName,
		Help: ServiceReconciliationErrorMetricDescription,
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
