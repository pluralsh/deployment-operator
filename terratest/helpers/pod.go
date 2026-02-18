package helpers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultPodContainerName = "default"
)

func ToPodJSON(pod *corev1.Pod) string {
	if pod == nil {
		return "{}"
	}

	marshalled, err := json.Marshal(pod)
	if err != nil {
		return "{}"
	}

	return string(marshalled)
}

func CreatePodForPVC(t *testing.T, options *k8s.KubectlOptions, name, pvcName string) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  defaultPodContainerName,
					Image: "busybox:1.36",
					Command: []string{
						"sh",
						"-c",
						"echo 'pvc-test' > /data/verify.txt && grep -x 'pvc-test' /data/verify.txt",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "pvc-data",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "pvc-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	err := k8s.KubectlApplyFromStringE(t, options, ToPodJSON(pod))
	if err != nil {
		t.Fatalf("failed to create pod %s/%s: %v", name, options.Namespace, err)
	}
}

func WaitForPodSucceeded(t *testing.T, options *k8s.KubectlOptions, podName string, timeout time.Duration) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			t.Fatalf("timeout waiting for pod %s/%s to succeed\n", podName, options.Namespace)
		case <-ticker.C:
			pod, err := k8s.GetPodE(t, options, podName)
			if err != nil {
				t.Logf("failed to get pod %s/%s: %v", podName, options.Namespace, err)
				continue
			}

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				return
			case corev1.PodFailed:
				logs := k8s.GetPodLogs(t, options, pod, defaultPodContainerName)
				t.Fatalf("pod %s/%s failed: %s\nlogs:\n%s\n", podName, options.Namespace, pod.Status.Reason, logs)
			}
		}
	}
}
