package metrics

const (
	DiscoveryAPICacheRefreshMetricName        = "agent_discoveryapi_cache_refresh_total"
	DiscoveryAPICacheRefreshMetricDescription = "The total number of Discovery API cache refresh attempts"

	DiscoveryAPICacheRefreshErrorMetricName        = "agent_discoveryapi_cache_refresh_error_total"
	DiscoveryAPICacheRefreshErrorMetricDescription = "The total number of Discovery API cache refresh errors"

	ServiceReconciliationMetricName        = "agent_service_reconciliations_total"
	ServiceReconciliationMetricDescription = "The total number of service reconciliations"

	ServiceReconciliationErrorMetricName        = "agent_service_reconciliation_errors_total"
	ServiceReconciliationErrorMetricDescription = "The total number of service reconciliation errors"

	ServiceReconciliationMetricLabelServiceName = "service_name"

	StackRunJobsCreatedMetricName        = "agent_stack_runs_created_total"
	StackRunJobsCreatedMetricDescription = "The total number of created stack runs"

	ResourceCacheOpenWatchesName              = "agent_resource_cache_open_watches_total"
	ResourceCacheOpenWatchesDescription       = "The total number of open watches in the resource cache"
	ResourceCacheOpenWatchesLabelResourceType = "resource_type"

	ResourceCacheHitMetricName        = "agent_resource_cache_hit_total"
	ResourceCacheHitMetricDescription = "The total number of resource cache hits"

	ResourceCacheMissMetricName        = "agent_resource_cache_miss_total"
	ResourceCacheMissMetricDescription = "The total number of resource cache misses"

	MetricLabelServiceID = "service_id"
)

type Recorder interface {
	DiscoveryAPICacheRefresh(err error)
	ServiceReconciliation(serviceID, serviceName string, err error)
	ServiceDeletion(serviceID string)
	StackRunJobCreation()
	ResourceCacheWatchStart(resourceType string)
	ResourceCacheWatchEnd(resourceType string)
	ResourceCacheHit(serviceID string)
	ResourceCacheMiss(serviceID string)
}
