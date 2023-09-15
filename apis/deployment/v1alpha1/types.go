package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applications,shortName=app;apps
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ApplicationSpec   `json:"spec"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

type ApplicationSpec struct {
	// PluralId is an ID of deployment from Plural.
	PluralId string `json:"pluralId"`

	// RepoUrl is a URL of repository to sync.
	RepoUrl string `json:"repoUrl"`

	// Ref to fetch from the repository.
	Ref string `json:"ref"`

	// Subfolder from the repository to work with.
	Subfolder string `json:"subfolder"`

	// Namespace to sync into.
	Namespace string `json:"namespace"`
}

type ApplicationStatus struct {
	// Ref shows current ref its synced to.
	Ref string `json:"ref"`

	Resources []ApplicationResource `json:"resources"`
}

type ApplicationResource struct {
	APIVersion string                    `json:"apiVersion"`
	Kind       string                    `json:"kind"`
	Name       string                    `json:"name"`
	Namespace  string                    `json:"namespace"`
	Synced     bool                      `json:"synced"`
	Status     ApplicationResourceStatus `json:"status"`
}

// ApplicationResourceStatus represents current status of application resource.
// +kubebuilder:validation:Enum=Pending;Failed;Succeeded
type ApplicationResourceStatus string

// ApplicationList is a list of Applications.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}
