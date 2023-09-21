/*
Copyright 2023.

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
	crhelperTypes "github.com/pluralsh/controller-reconcile-helper/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Deployment{}, &DeploymentList{})
}

const (
	// DeploymentReadyCondition used when deployment is ready.
	DeploymentReadyCondition crhelperTypes.ConditionType = "DeploymentReady"

	// FailedToCreateDeploymentReason used when grpc method for deployment creation failed.
	FailedToCreateDeploymentReason = "FailedToCreateDeployment"
)

type Revision struct {
	// Git ia a spec of the prior revision
	Git     GitRef `json:",inline"`
	Version string `json:"version"`
}

type GitRef struct {
	// Ref is the URL to the repository (Git or Helm) that contains the application manifests
	Ref string `json:"ref"`
	// Folder from the repository to work with.
	Folder string `json:"folder,omitempty"`
}

type DeploymentSpec struct {

	// PluralId is an ID of deployment from Plural.
	PluralId string `json:"pluralId"`

	// Revision is revision to sync.
	Revision Revision `json:"revision"`

	// Git ia a spec of the current revision
	Git GitRef `json:"git"`

	// Namespace to sync into.
	Namespace string `json:"namespace"`

	// ProviderName is the name of provider associated with this deployment operator
	ProviderName string `json:"providerName"`

	// Name of the DeploymentClass
	DeploymentClassName string `json:"deploymentClassName"`

	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// ExistingDeploymentID is the unique id of the deployment.
	// This field will be empty when the Deployment is dynamically provisioned by operator.
	// +optional
	ExistingDeploymentID string `json:"existingDeploymentID,omitempty"`

	// DeletionPolicy is used to specify how to handle deletion. There are 2 possible values:
	//  - Retain: Indicates that the Deployment should not be deleted (default)
	//  - Delete: Indicates that the Deployment should be deleted
	//
	// +optional
	// +kubebuilder:default:=Retain
	DeletionPolicy DeletionPolicy `json:"deletionPolicy"`
}

type DeploymentStatus struct {
	// Ref shows current ref its synced to.
	Ref string `json:"ref"`

	Resources []DeploymentResource `json:"resources"`

	// Ready is a boolean condition to reflect the successful creation of a deployment.
	Ready bool `json:"ready,omitempty"`

	// DeploymentID is the unique id of the deployment.
	// +optional
	DeploymentID string `json:"deploymentID,omitempty"`

	// Conditions defines current state.
	// +optional
	Conditions crhelperTypes.Conditions `json:"conditions,omitempty"`
}

type DeploymentResource struct {
	APIVersion string                   `json:"apiVersion"`
	Kind       string                   `json:"kind"`
	Name       string                   `json:"name"`
	Namespace  string                   `json:"namespace"`
	Synced     bool                     `json:"synced"`
	Status     DeploymentResourceStatus `json:"status"`
}

// DeploymentResourceStatus represents current status of application resource.
// +kubebuilder:validation:Enum=Pending;Failed;Succeeded
type DeploymentResourceStatus string

// GetConditions returns the list of conditions for a WireGuardServer API object.
func (d *Deployment) GetConditions() crhelperTypes.Conditions {
	return d.Status.Conditions
}

// SetConditions will set the given conditions on a WireGuardServer object.
func (d *Deployment) SetConditions(conditions crhelperTypes.Conditions) {
	d.Status.Conditions = conditions
}

// Deployment is a definition of Deployment resource.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Deployment ready status"
type Deployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              DeploymentSpec   `json:"spec"`
	Status            DeploymentStatus `json:"status,omitempty"`
}

// DeploymentList is a list of Deployments.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Deployment `json:"items"`
}
