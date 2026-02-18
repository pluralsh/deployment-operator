package helpers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ToPersistentVolumeClaimJSON(pvc *corev1.PersistentVolumeClaim) string {
	if pvc == nil {
		return "{}"
	}

	marshalled, err := json.Marshal(pvc)
	if err != nil {
		return "{}"
	}

	return string(marshalled)
}

func CreatePersistentVolumeClaim(t *testing.T, options *k8s.KubectlOptions, name, storageClass string, quantity resource.Quantity) {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
			StorageClassName: lo.ToPtr(storageClass),
		},
	}

	err := k8s.KubectlApplyFromStringE(t, options, ToPersistentVolumeClaimJSON(pvc))
	if err != nil {
		t.Fatalf("failed to create pvc %s/%s: %v", name, options.Namespace, err)
	}
}

func WaitForPVCBound(t *testing.T, options *k8s.KubectlOptions, namespace, name string, timeout time.Duration) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeout):
			t.Fatalf("timed out waiting for pvc %s/%s to be bound", namespace, name)
		case <-ticker.C:
			pvc, err := k8s.GetPersistentVolumeClaimE(t, options, name)
			if err != nil {
				t.Logf("failed to get pvc %s/%s: %v", namespace, name, err)
				continue
			}

			if pvc.Status.Phase == corev1.ClaimBound {
				return
			}
		}
	}
}
