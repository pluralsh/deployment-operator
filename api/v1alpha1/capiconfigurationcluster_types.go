package v1alpha1

import (
	"context"
	"fmt"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	SchemeBuilder.Register(&CapiConfigurationClusterList{}, &CapiConfigurationCluster{})
}

// CapiConfigurationClusterList contains a list of [CapiConfigurationCluster]
// +kubebuilder:object:root=true
type CapiConfigurationClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CapiConfigurationCluster `json:"items"`
}

// CapiConfigurationCluster is the Schema for the CAPI cluster configuration
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
type CapiConfigurationCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec of the CAPI cluster configuration
	// +kubebuilder:validation:Required
	Spec CapiConfigurationClusterSpec `json:"spec"`

	// Status of the CAPI cluster configuration
	// +kubebuilder:validation:Optional
	Status Status `json:"status,omitempty"`
}

type CapiConfigurationClusterSpec struct {
	// Cluster is a simplified representation of the Console API cluster
	// object. See [ClusterSpec] for more information.
	// +kubebuilder:validation:Optional
	Cluster *ClusterSpec `json:"cluster,omitempty"`

	// TokenSecretRef contains the reference to the secret holding the token to access the Console API
	// +kubebuilder:validation:Required
	ConsoleTokenSecretRef corev1.SecretKeySelector `json:"consoleTokenSecretRef"`

	// Cluster contains the reference to the CAPI cluster
	// +kubebuilder:validation:Required
	CapiCluster corev1.ObjectReference `json:"capiCluster"`
}

func (in *CapiConfigurationCluster) SetCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&in.Status.Conditions, condition)
}

func (in *CapiConfigurationCluster) ClusterName() string {
	if in.Spec.Cluster != nil && in.Spec.Cluster.Handle != nil {
		return lo.FromPtr(in.Spec.Cluster.Handle)
	}
	return in.Spec.CapiCluster.Name
}

func (in *CapiConfigurationCluster) GetConsoleToken(ctx context.Context, c k8sClient.Client) (string, error) {
	secret := &corev1.Secret{}

	if err := c.Get(
		ctx,
		k8sClient.ObjectKey{Name: in.Spec.ConsoleTokenSecretRef.Name, Namespace: in.Namespace},
		secret,
	); err != nil {
		return "", err
	}

	token, exists := secret.Data[in.Spec.ConsoleTokenSecretRef.Key]
	if !exists {
		return "", fmt.Errorf("secret %s/%s does not contain console token", in.Namespace, in.Spec.ConsoleTokenSecretRef.Name)
	}

	return string(token), nil
}
