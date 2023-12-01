/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	console "github.com/pluralsh/console-client-go"
	batchv1 "k8s.io/api/batch/v1"
)

// +kubebuilder:validation:Enum=PENDING;OPEN;CLOSED
// GateState represents the state of a gate
// type GateState string
type GateState console.GateState

//const (
//	Pending GateState = "PENDING"
//	Open    GateState = "OPEN"
//	Closed  GateState = "CLOSED"
//)

// +kubebuilder:validation:Enum=APPROVAL;WINDOW;JOB
// GateType represents the type of a gate
// type GateType string
type GateType console.GateType

//const (
//	Approval GateType = "APPROVAL"
//	Window   GateType = "WINDOW"
//	Job      GateType = "JOB"
//)

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PipelineGate is the Schema for the pipelinegate API
// PipelineGate represents a gate blocking promotion along a release pipeline
type PipelineGate struct {
	metav1.TypeMeta   `json:",inline"`            // kind and apiVersion
	metav1.ObjectMeta `json:"metadata,omitempty"` // name, namespace, labels, annotations

	Spec   PipelineGateSpec   `json:"spec,omitempty"`
	Status PipelineGateStatus `json:"status,omitempty"`
}

// PipelineGateStatus defines the observed state of ConfigurationOverlay
type PipelineGateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State GateState `json:"state"`
}

// PipelineGateSpec defines the detailed gate specifications
type PipelineGateSpec struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Type     GateType  `json:"type"`
	GateSpec *GateSpec `json:"gateSpec,omitempty"`
}

// GateSpec defines the detailed gate specifications
type GateSpec struct {
	// TODO: think about replacing this with the actual type from the k8s api
	JobSpec *batchv1.JobSpec `json:"job"`
}

//// JobGateSpec represents the full specification of a job gate
//type JobGateSpec struct {
//	Namespace  string          `json:"namespace"`
//	Raw        string          `json:"raw"`
//	Containers []ContainerSpec `json:"containers"`
//	// TODO: add support for these
//	//Labels         Map             `json:"labels"`
//	//Annotations    Map             `json:"annotations"`
//	ServiceAccount string `json:"serviceAccount"`
//}
//
//// ContainerSpec represents a shortform spec for job containers, designed for ease-of-use
//type ContainerSpec struct {
//	Image   string             `json:"image"`
//	Args    []string           `json:"args"`
//	Env     []ContainerEnv     `json:"env"`
//	EnvFrom []ContainerEnvFrom `json:"envFrom"`
//}
//
//// ContainerEnv represents a container env variable
//type ContainerEnv struct {
//	Name  string `json:"name"`
//	Value string `json:"value"`
//}
//
//// ContainerEnvFrom represents env from declarations for containers
//type ContainerEnvFrom struct {
//	ConfigMap string `json:"configMap"`
//	Secret    string `json:"secret"`
//}

// +kubebuilder:object:root=true
// PipelineGateList contains a list of PipelineGate
type PipelineGateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineGate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PipelineGate{}, &PipelineGateList{})
}
