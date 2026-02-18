package helpers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ToServiceJSON(service *corev1.Service) string {
	if service == nil {
		return "{}"
	}

	marshalled, err := json.Marshal(service)
	if err != nil {
		return "{}"
	}

	return string(marshalled)
}

func CreateLoadBalancerServiceWithCleanup(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, name string, selector, labels, annotations map[string]any, port int32) {
	CreateLoadBalancerService(t, options, name, selector, labels, annotations, port)
	t.Cleanup(func() { DeleteService(ctx, t, options, name) })
}

func CreateLoadBalancerService(t *testing.T, options *k8s.KubectlOptions, name string, selector, labels, annotations map[string]any, port int32) {
	mergedLabels := MergeLabels(selector, labels)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   options.Namespace,
			Labels:      ToStringMap(mergedLabels),
			Annotations: ToStringMap(annotations),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeLoadBalancer,
			Selector: ToStringMap(selector),
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: port},
				},
			},
		},
	}

	if err := k8s.KubectlApplyFromStringE(t, options, ToServiceJSON(service)); err != nil {
		t.Fatalf("failed to create load balancer service %s/%s: %v", options.Namespace, name, err)
	}
}

func WaitForServiceLoadBalancerReady(t *testing.T, options *k8s.KubectlOptions, name string, timeout time.Duration) *corev1.Service {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		t.Fatalf("failed to get kubernetes client: %v", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			t.Fatalf("timeout waiting for load balancer service %s/%s to be ready", options.Namespace, name)
		case <-ticker.C:
			service, err := clientset.CoreV1().Services(options.Namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				t.Logf("failed to get service %s/%s: %v", options.Namespace, name, err)
				continue
			}

			if len(service.Status.LoadBalancer.Ingress) > 0 {
				return service
			}
		}
	}
}

func DeleteService(ctx context.Context, t *testing.T, options *k8s.KubectlOptions, name string) {
	clientset, err := k8s.GetKubernetesClientFromOptionsE(t, options)
	if err != nil {
		t.Logf("failed to get kubernetes client for service delete: %v", err)
		return
	}

	if err := clientset.CoreV1().Services(options.Namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("failed to delete service %s/%s: %v", options.Namespace, name, err)
	}
}
