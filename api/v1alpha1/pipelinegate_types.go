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
	"fmt"

	"github.com/google/uuid"
	console "github.com/pluralsh/console-client-go"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=PENDING;OPEN;CLOSED
// GateState represents the state of a gate, reused from console client
type GateState console.GateState

// +kubebuilder:validation:Enum=APPROVAL;WINDOW;JOB
// GateType represents the type of a gate, reused from console client
type GateType console.GateType

//+genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PipelineGate represents a gate blocking promotion along a release pipeline
type PipelineGate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineGateSpec   `json:"spec,omitempty"`
	Status PipelineGateStatus `json:"status,omitempty"`
}

// PipelineGateStatus defines the observed state of the PipelineGate
type PipelineGateStatus struct {
	State *GateState `json:"state,omitempty"`
	//SyncedState    GateState               `json:"syncedState"`
	//LastSyncedAt   metav1.Time             `json:"lastSyncedAt,omitempty"`
	//LastReported   *GateState              `json:"lastReported,omitempty"`
	//LastReportedAt *metav1.Time            `json:"lastReportedAt,omitempty"`
	JobRef *console.NamespacedName `json:"jobRef,omitempty"`
	SHA    *string                 `json:"sha,omitempty"`
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
	// resuse JobSpec type from the kubernetes api
	JobSpec *batchv1.JobSpec `json:"job"`
}

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

func (pgs *PipelineGateStatus) IsInitialized() bool {
	return pgs.State != nil
}

func (pgs *PipelineGateStatus) IsPending() bool {
	return pgs.State != nil && *pgs.State == GateState(console.GateStatePending)
}

func (pgs *PipelineGateStatus) IsOpen() bool {
	return pgs.State != nil && *pgs.State == GateState(console.GateStateOpen)
}

func (pgs *PipelineGateStatus) IsClosed() bool {
	return pgs.State != nil && *pgs.State == GateState(console.GateStateClosed)
}

func (pgs *PipelineGateStatus) HasJobRef() bool {
	return !(pgs.JobRef == nil || *pgs.JobRef == console.NamespacedName{})
}

//func (pgs *PipelineGateStatus) HasNotReported() bool {
//	return pgs.LastReported == nil || (pgs.LastReportedAt != nil && pgs.LastReportedAt.Before(&pgs.LastSyncedAt))
//}

func (pgs *PipelineGateStatus) GetConsoleGateState() (*console.GateState, error) {
	if pgs.State == nil {
		return nil, fmt.Errorf("gate state is not initialized")
	}
	state := console.GateState(*pgs.State)
	return &state, nil
}

func (pgs *PipelineGateStatus) GetSHA() string {
	if !pgs.HasSHA() {
		return ""
	}
	return *pgs.SHA
}

func (pgs *PipelineGateStatus) HasSHA() bool {
	return pgs.SHA != nil && len(*pgs.SHA) > 0
}

func (pgs *PipelineGateStatus) IsSHAEqual(sha string) bool {
	if !pgs.HasSHA() {
		return false
	}
	return pgs.GetSHA() == sha
}

func (pgs *PipelineGateStatus) SetState(state console.GateState) *PipelineGateStatus {
	gateState := GateState(state)
	pgs.State = &gateState
	return pgs
}

func (pgs *PipelineGateStatus) SetJobRef(name string, namespace string) *PipelineGateStatus {
	nsn := console.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	pgs.JobRef = &nsn
	return pgs
}

func (pgs *PipelineGateStatus) GateUpdateAttributes() (*console.GateUpdateAttributes, error) {
	state, err := pgs.GetConsoleGateState()
	if err != nil {
		return nil, err
	}
	updateAttributes := console.GateUpdateAttributes{State: state, Status: &console.GateStatusAttributes{JobRef: pgs.JobRef}}
	return &updateAttributes, nil
}

func (pgs *PipelineGateStatus) SetSHA(sha string) *PipelineGateStatus {
	pgs.SHA = &sha
	return pgs
}

func (pg *PipelineGate) CreateNewJobName() string {
	jobName := fmt.Sprintf("%s-%s", pg.Name, uuid.New().String()[:8])
	return jobName
}

func (pg *PipelineGate) CreateNewJobRef() console.NamespacedName {
	jobRef := console.NamespacedName{Name: pg.CreateNewJobName(), Namespace: pg.Namespace}
	if pg.Spec.GateSpec != nil && pg.Spec.GateSpec.JobSpec != nil && pg.Spec.GateSpec.JobSpec.Template.Namespace != "" {
		pg.Namespace = pg.Spec.GateSpec.JobSpec.Template.Namespace
	}
	return jobRef
}
