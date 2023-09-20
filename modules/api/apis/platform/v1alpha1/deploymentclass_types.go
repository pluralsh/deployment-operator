/*
Copyright 2022.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&DeploymentClass{}, &DeploymentClassList{})
}

type DeletionPolicy string

const (
	DeletionPolicyRetain DeletionPolicy = "Retain"
	DeletionPolicyDelete DeletionPolicy = "Delete"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type DeploymentClass struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DriverName is the name of driver associated with this deployment
	DriverName string `json:"driverName"`

	// Parameters is an opaque map for passing in configuration to a driver
	// for creating the deployment
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// DeletionPolicy is used to specify how to handle deletion. There are 2 possible values:
	//  - Retain: Indicates that the deployment should not be deleted (default)
	//  - Delete: Indicates that the deployment should be deleted
	//
	// +optional
	// +kubebuilder:default:=Retain
	DeletionPolicy DeletionPolicy `json:"deletionPolicy"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DeploymentClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentClass `json:"items"`
}
