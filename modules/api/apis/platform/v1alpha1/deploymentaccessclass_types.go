package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&DeploymentAccessClass{}, &DeploymentAccessClassList{})
}

type AuthenticationType string

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type DeploymentAccessClass struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DriverName is the name of driver associated with
	// this DeploymentAccess
	DriverName string `json:"driverName"`

	// AuthenticationType denotes the style of authentication
	AuthenticationType AuthenticationType `json:"authenticationType"`

	// Parameters is an opaque map for passing in configuration to a driver
	// for granting access to the deployment
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DeploymentAccessClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeploymentAccessClass `json:"items"`
}
