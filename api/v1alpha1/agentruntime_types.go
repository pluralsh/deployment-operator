package v1alpha1

import (
	console "github.com/pluralsh/console/go/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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

	// Template defines the pod template for this agent runtime.
	// +kubebuilder:validation:Required
	Template corev1.PodTemplateSpec `json:"template"`

	// Config contains typed configuration depending on the chosen runtime type.
	Config AgentRuntimeConfig `json:"config"`
}

// AgentRuntimeConfig contains typed configuration for the agent runtime.
type AgentRuntimeConfig struct {
	// Config for Claude CLI runtime.
	// +kubebuilder:validation:Optional
	Claude *ClaudeConfig `json:"claude,omitempty"`

	// Config for OpenCode CLI runtime.
	// +kubebuilder:validation:Optional
	OpenCode *OpenCodeConfig `json:"opencode,omitempty"`
}

// ClaudeConfig contains configuration for the Claude CLI runtime.
type ClaudeConfig struct {
	// ApiKeySecretRef Reference to a Kubernetes Secret containing the Claude API key.
	ApiKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`

	// Model Name of the model to use.
	Model *string `json:"model,omitempty"`

	// ExtraArgs CLI args for advanced flags not modeled here
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// OpenCodeConfig contains configuration for the OpenCode CLI runtime.
type OpenCodeConfig struct {
	// API endpoint for the OpenCode service.
	Endpoint string `json:"endpoint"`

	// Reference to a Kubernetes Secret containing the API token for OpenCode.
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`

	// Extra args for advanced or experimental CLI flags.
	ExtraArgs []string `json:"extraArgs,omitempty"`
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

func (in *AgentRuntime) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}

// ConsoleID implements [PluralResource] interface
func (in *AgentRuntime) ConsoleID() *string {
	return in.Status.ID
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
