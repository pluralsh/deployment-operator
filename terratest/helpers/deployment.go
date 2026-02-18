package helpers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultDeploymentContainerName = "app"
)

func ToDeploymentJSON(deployment *appsv1.Deployment) string {
	if deployment == nil {
		return "{}"
	}

	marshalled, err := json.Marshal(deployment)
	if err != nil {
		return "{}"
	}

	return string(marshalled)
}

func CreateDeploymentWithCleanup(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, name string, labels map[string]any, image string, port int32) {
	CreateDeployment(t, options, name, labels, image, port)
	t.Cleanup(func() { DeleteDeployment(ctx, t, options, name) })
}

func CreateDeployment(t *testing.T, options *k8s.KubectlOptions, name string, labels map[string]any, image string, port int32) {
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
			Labels:    ToStringMap(labels),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: ToStringMap(labels)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: ToStringMap(labels)},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultDeploymentContainerName,
							Image: image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: port},
							},
						},
					},
				},
			},
		},
	}

	if err := k8s.KubectlApplyFromStringE(t, options, ToDeploymentJSON(deployment)); err != nil {
		t.Fatalf("failed to create deployment %s/%s: %v", name, options.Namespace, err)
	}
}

func WaitForDeploymentReady(t *testing.T, options *k8s.KubectlOptions, name string, timeout time.Duration) {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		t.Fatalf("failed to get kubernetes client: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeout):
			t.Fatalf("timeout waiting for deployment %s/%s to be ready", options.Namespace, name)
		case <-ticker.C:
			deployment, err := clientset.AppsV1().Deployments(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				t.Logf("failed to get deployment %s/%s: %v", options.Namespace, name, err)
				continue
			}

			desired := int32(1)
			if deployment.Spec.Replicas != nil {
				desired = *deployment.Spec.Replicas
			}

			if deployment.Status.AvailableReplicas >= desired {
				return
			}
		}
	}
}

func DeleteDeployment(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, name string) {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		t.Logf("failed to get kubernetes client for deployment delete: %v", err)
		return
	}

	if err := clientset.AppsV1().Deployments(options.Namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("failed to delete deployment %s/%s: %v", options.Namespace, name, err)
	}
}
