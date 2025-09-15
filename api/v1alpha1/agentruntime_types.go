package v1alpha1

import (
	console "github.com/pluralsh/console/go/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentRuntimeSpec defines the desired state of AgentRuntime
type AgentRuntimeSpec struct {
	// Name of this AgentRuntime.
	// If not provided, the name from AgentRuntime.ObjectMeta will be used.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`

	// Type specifies the agent runtime to use for executing the stack.
	// One of CLAUDE, OPENCODE.
	// +kubebuilder:validation:Enum=CLAUDE;OPENCODE
	// +kubebuilder:validation:Required
	Type console.AgentRuntimeType `json:"type"`

	// Bindings define the creation permissions for this agent runtime.
	// +kubebuilder:validation:Optional
	Bindings *AgentRuntimeBindings `json:"bindings,omitempty"`
}

type AgentRuntimeBindings struct {
	// Create bindings control who can generate new agent runtimes.
	// +kubebuilder:validation:Optional
	Create []Binding `json:"create,omitempty"`
}

func (in *AgentRuntime) ConsoleName() string {
	if in.Spec.Name != nil && len(*in.Spec.Name) > 0 {
		return *in.Spec.Name
	}

	return in.Name
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// AgentRuntime is the Schema for the agentruntimes API
type AgentRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentRuntimeSpec `json:"spec,omitempty"`
	Status Status           `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentRuntimeList contains a list of AgentRuntime
type AgentRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentRuntime `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentRuntime{}, &AgentRuntimeList{})
}
