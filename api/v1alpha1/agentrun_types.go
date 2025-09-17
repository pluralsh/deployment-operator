package v1alpha1

import (
	console "github.com/pluralsh/console/go/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentRunSpec defines the desired state of AgentRun
type AgentRunSpec struct {
	// RuntimeRef references the AgentRuntime that should be used for this run
	// +kubebuilder:validation:Required
	RuntimeRef corev1.ObjectReference `json:"runtimeRef"`

	// Prompt is the task/prompt given to the agent
	// +kubebuilder:validation:Required
	Prompt string `json:"prompt"`

	// Repository is the git repository the agent will work with
	// +kubebuilder:validation:Required
	Repository string `json:"repository"`

	// Mode defines how the agent should run (ANALYZE, WRITE)
	// +kubebuilder:validation:Required
	Mode console.AgentRunMode `json:"mode"`

	// FlowID is the flow this agent run is associated with (optional)
	// +kubebuilder:validation:Optional
	FlowID *string `json:"flowId,omitempty"`
}

// AgentRunStatus defines the observed state of AgentRun
type AgentRunStatus struct {
	// PodRef references the pod running this agent
	// +kubebuilder:validation:Optional
	PodRef *corev1.ObjectReference `json:"podRef,omitempty"`

	// Phase represents the current phase of the agent run
	// +kubebuilder:validation:Optional
	Phase AgentRunPhase `json:"phase,omitempty"`

	// StartTime is when the agent run started
	// +kubebuilder:validation:Optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// EndTime is when the agent run completed
	// +kubebuilder:validation:Optional
	EndTime *metav1.Time `json:"endTime,omitempty"`

	// Error message if the run failed
	// +kubebuilder:validation:Optional
	Error *string `json:"error,omitempty"`

	// Standard status fields
	Status `json:",inline"`
}

// AgentRunPhase represents the phase of an agent run
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed
type AgentRunPhase string

const (
	AgentRunPhasePending   AgentRunPhase = "Pending"
	AgentRunPhaseRunning   AgentRunPhase = "Running"
	AgentRunPhaseSucceeded AgentRunPhase = "Succeeded"
	AgentRunPhaseFailed    AgentRunPhase = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// AgentRun is the Schema for the agentruns API
type AgentRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentRunSpec   `json:"spec,omitempty"`
	Status AgentRunStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AgentRunList contains a list of AgentRun
type AgentRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentRun{}, &AgentRunList{})
}

// Helper methods
func (ar *AgentRun) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&ar.Status.Conditions, condition)
}

func (ar *AgentRun) IsPending() bool {
	return ar.Status.Phase == AgentRunPhasePending
}

func (ar *AgentRun) IsRunning() bool {
	return ar.Status.Phase == AgentRunPhaseRunning
}

func (ar *AgentRun) IsSucceeded() bool {
	return ar.Status.Phase == AgentRunPhaseSucceeded
}

func (ar *AgentRun) IsFailed() bool {
	return ar.Status.Phase == AgentRunPhaseFailed
}
