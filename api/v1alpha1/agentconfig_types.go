package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AgentConfigurationSpec struct {
	ServicePollInterval               *string `json:"servicePollInterval,omitempty"`
	ClusterPingInterval               *string `json:"clusterPingInterval,omitempty"`
	CompatibilityUploadInterval       *string `json:"compatibilityUploadInterval,omitempty"`
	StackPollInterval                 *string `json:"stackPollInterval,omitempty"`
	PipelineGateInterval              *string `json:"pipelineGateInterval,omitempty"`
	MaxConcurrentReconciles           *int    `json:"maxConcurrentReconciles,omitempty"`
	VulnerabilityReportUploadInterval *string `json:"vulnerabilityReportUploadInterval,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// AgentConfiguration is the deployment operator configuration
type AgentConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentConfigurationSpec `json:"spec,omitempty"`
	Status Status                 `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentConfigurationList contains a list of AgentConfiguration
type AgentConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentConfiguration{}, &AgentConfigurationList{})
}

func (c *AgentConfiguration) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&c.Status.Conditions, condition)
}
