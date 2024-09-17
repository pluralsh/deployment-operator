package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&MetricsAggregate{}, &MetricsAggregateList{})
}

// MetricsAggregateList contains a list of [MetricsAggregate]
// +kubebuilder:object:root=true
type MetricsAggregateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MetricsAggregate `json:"items"`
}

// MetricsAggregate
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
type MetricsAggregate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec of the MetricsAggregate
	// +kubebuilder:validation:Required
	Spec MetricsAggregateSpec `json:"spec"`

	// Status of the IngressReplica
	// +kubebuilder:validation:Optional
	Status Status `json:"status,omitempty"`
}

type MetricsAggregateSpec struct {
	Nodes int `json:"nodes"`
	// MemoryTotalBytes current memory usage in bytes
	MemoryTotalBytes int64 `json:"memoryTotalBytes,omitempty"`
	// MemoryAvailableBytes available memory for node
	MemoryAvailableBytes int64 `json:"memoryAvailableBytes,omitempty"`
	// MemoryUsedPercentage in percentage
	MemoryUsedPercentage int64 `json:"memoryUsedPercentage,omitempty"`
	// CPUTotalMillicores in m cores
	CPUTotalMillicores int64 `json:"cpuTotalMillicores,omitempty"`
	// CPUAvailableMillicores in m cores
	CPUAvailableMillicores int64 `json:"cpuAvailableMillicores,omitempty"`
	// CPUUsedPercentage in percentage
	CPUUsedPercentage int64 `json:"cpuUsedPercentage,omitempty"`
}

func (in *MetricsAggregate) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}
