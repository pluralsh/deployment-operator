package metrics

const (
	DiscoveryAPICacheRefreshMetricName        = "agent_discoveryapi_cache_refresh_total"
	DiscoveryAPICacheRefreshMetricDescription = "The total number of Discovery API cache refreshes"

	ServiceReconciliationMetricName             = "agent_service_reconciliations_total"
	ServiceReconciliationMetricDescription      = "The total number of service reconciliations"
	ServiceReconciliationMetricLabelServiceID   = "service_id"
	ServiceReconciliationMetricLabelServiceName = "service_name"

	StackRunJobsCreatedMetricName        = "agent_stack_runs_created_total"
	StackRunJobsCreatedMetricDescription = "The total number of created stack runs"
)

type Recorder interface {
	DiscoveryAPICacheRefresh()
	ServiceReconciliation(serviceID, serviceName string)
	StackRunJobCreation()
}
