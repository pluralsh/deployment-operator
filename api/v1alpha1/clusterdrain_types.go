package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FlowControl struct {
	Percentage     *int `json:"percentage,omitempty"`
	MaxConcurrency *int `json:"maxConcurrency,omitempty"`
}

// ClusterDrainSpec defines the desired state of ClusterDrain
type ClusterDrainSpec struct {
	FlowControl   FlowControl           `json:"flowControl"`
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// ClusterDrainStatus defines the observed state of ClusterDrain
type ClusterDrainStatus struct {
	// Represents the observations of a HealthConvert current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	Progress   []Progress         `json:"progress,omitempty"`
}

type Progress struct {
	Wave       int                      `json:"wave"`
	Percentage int                      `json:"percentage"`
	Count      int                      `json:"count"`
	Failures   []corev1.ObjectReference `json:"failures,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterDrain is the Schema for the ClusterDrain object
type ClusterDrain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterDrainSpec   `json:"spec,omitempty"`
	Status ClusterDrainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterDrainList contains a list of ClusterDrain
type ClusterDrainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CustomHealth `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterDrain{}, &ClusterDrainList{})
}

func (c *ClusterDrain) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&c.Status.Conditions, condition)
}
