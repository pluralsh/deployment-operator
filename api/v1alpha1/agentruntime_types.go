package v1alpha1

import (
	"fmt"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const AgentRuntimeNameLabel = "deployments.plural.sh/agent-runtime-name"

// AgentRuntimeSpec defines the desired state of AgentRuntime
type AgentRuntimeSpec struct {
	// Name of this AgentRuntime.
	// If not provided, the name from AgentRuntime.ObjectMeta will be used.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`

	// +kubebuilder:validation:Required
	TargetNamespace string `json:"targetNamespace"`

	// Type specifies the agent runtime to use for executing the stack.
	// One of CLAUDE, OPENCODE, GEMINI, CUSTOM.
	// +kubebuilder:validation:Enum=CLAUDE;OPENCODE;GEMINI;CUSTOM
	// +kubebuilder:validation:Required
	Type console.AgentRuntimeType `json:"type"`

	// Bindings define the creation permissions for this agent runtime.
	// +kubebuilder:validation:Optional
	Bindings *AgentRuntimeBindings `json:"bindings,omitempty"`

	// Template defines the pod template for this agent runtime.
	Template *corev1.PodTemplateSpec `json:"template,omitempty"`

	// Config contains typed configuration depending on the chosen runtime type.
	// +kubebuilder:validation:Optional
	Config *AgentRuntimeConfig `json:"config,omitempty"`

	// AiProxy specifies whether the agent runtime should be proxied through the AI proxy.
	AiProxy *bool `json:"aiProxy,omitempty"`
}

type PodTemplateSpec struct {
	// Labels to apply to the job for organization and selection.
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to apply to the job for additional metadata.
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec corev1.PodSpec `json:"spec,omitempty"`
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

func (in *AgentRuntimeConfig) ToAgentRuntimeConfigRaw(secretGetter func(corev1.SecretKeySelector) (*corev1.Secret, error)) (*AgentRuntimeConfigRaw, error) {
	openCode, err := in.OpenCode.ToOpenCodeConfigRaw(secretGetter)
	if err != nil {
		return nil, err
	}

	return &AgentRuntimeConfigRaw{
		Claude:   nil,
		OpenCode: openCode,
	}, nil
}

// AgentRuntimeConfigRaw contains raw configuration for the agent runtime.
//
// NOTE: Do not embed this struct directly, use AgentRuntimeConfig instead.
// This is only used to read original AgentRuntimeConfig secret data and be
// able to inject it into the pod as env vars.
type AgentRuntimeConfigRaw struct {
	// Claude is the raw configuration for the Claude runtime.
	// +kubebuilder:validation:Optional
	Claude *ClaudeConfigRaw `json:"claude,omitempty"`

	// OpenCode is the raw configuration for the OpenCode runtime.
	// +kubebuilder:validation:Optional
	OpenCode *OpenCodeConfigRaw `json:"opencode,omitempty"`
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

// ClaudeConfigRaw contains configuration for the Claude CLI runtime.
//
// NOTE: Do not embed this struct directly, use ClaudeConfig instead.
// This is only used to read original ClaudeConfig secret data and be
// able to inject it into the pod as env vars.
type ClaudeConfigRaw struct {
	// ApiKey is the raw API key to use.
	ApiKey string `json:"apiKey"`

	// Model Name of the model to use.
	Model *string `json:"model,omitempty"`

	// ExtraArgs CLI args for advanced flags not modeled here
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// OpenCodeConfig contains configuration for the OpenCode CLI runtime.
type OpenCodeConfig struct {
	// Provider is the OpenCode provider to use.
	// +kubebuilder:validation:Enum=plural;openai
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`

	// Endpoint API endpoint for the OpenCode service.
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// Model is the LLM model to use.
	// +kubebuilder:validation:Optional
	Model *string `json:"model,omitempty"`

	// TokenSecretRef is a reference to a Kubernetes Secret containing the API token for OpenCode.
	// +kubebuilder:validation:Required
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`

	// ExtraArgs args for advanced or experimental CLI flags.
	// Deprecated: It is being ignored by the agent harness.
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

func (in *OpenCodeConfig) ToOpenCodeConfigRaw(secretGetter func(corev1.SecretKeySelector) (*corev1.Secret, error)) (*OpenCodeConfigRaw, error) {
	tokenSecret, err := secretGetter(in.TokenSecretRef)
	if err != nil {
		return nil, err
	}

	token, exists := tokenSecret.Data[in.TokenSecretRef.Key]
	if !exists {
		return nil, fmt.Errorf("token secret does not contain key %s", in.TokenSecretRef.Key)
	}

	return &OpenCodeConfigRaw{
		Provider: in.Provider,
		Endpoint: in.Endpoint,
		Model:    in.Model,
		Token:    string(token),
	}, nil
}

// OpenCodeConfigRaw contains configuration for the OpenCode CLI runtime.
//
// NOTE: Do not embed this struct directly, use OpenCodeConfig instead.
// This is only used to read original OpenCodeConfig secret data and be
// able to inject it into the pod as env vars.
type OpenCodeConfigRaw struct {
	// Provider is the OpenCode provider to use.
	Provider string `json:"provider"`

	// Endpoint API endpoint for the OpenCode service.
	Endpoint string `json:"endpoint"`

	// Model is the LLM model to use.
	Model *string `json:"model,omitempty"`

	// Token is the raw API token for OpenCode.
	Token string `json:"tokenSecretRef"`
}

type AgentRuntimeBindings struct {
	// Create bindings control who can generate new agent runtimes.
	// +kubebuilder:validation:Optional
	Create []Binding `json:"create,omitempty"`
}

func (in *AgentRuntime) Diff(hasher Hasher) (changed bool, sha string, err error) {
	currentSha, err := hasher(in.Spec)
	if err != nil {
		return false, "", err
	}

	return !in.Status.IsSHAEqual(currentSha), currentSha, nil
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

func (in *AgentRuntime) Attributes() console.AgentRuntimeAttributes {
	attrs := console.AgentRuntimeAttributes{
		Name:    in.ConsoleName(),
		Type:    in.Spec.Type,
		AiProxy: in.Spec.AiProxy,
	}
	if in.Spec.Bindings != nil {
		attrs.CreateBindings = algorithms.Map(in.Spec.Bindings.Create, func(b Binding) *console.AgentBindingAttributes {
			return &console.AgentBindingAttributes{
				UserEmail: b.UserEmail,
				GroupName: b.GroupName,
			}
		})
	}
	return attrs
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Id",type="string",JSONPath=".status.id",description="Console ID"

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
