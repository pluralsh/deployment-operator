package stacks

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultJobContainer  = "default"
	defaultJobVolume     = "default"
	defaultJobVolumePath = "/harness"
)

func (r *StackReconciler) defaultJob(name string, run *console.StackRunFragment) batchv1.JobSpec {
	return batchv1.JobSpec{
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: r.Namespace,
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyOnFailure,
				Containers: []corev1.Container{
					{
						Name:            defaultJobContainer,
						Image:           r.defaultJobContainerImage(run.Configuration),
						ImagePullPolicy: corev1.PullAlways,
						Args:            r.defaultJobContainerArgs(run.ID),
						VolumeMounts: []corev1.VolumeMount{{
							Name:      defaultJobVolume,
							MountPath: defaultJobVolumePath,
						}},
					},
				},
				Volumes: []corev1.Volume{{
					Name: defaultJobVolume,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}},
			},
		},
	}
}

func (r *StackReconciler) defaultJobContainerImage(configuration *console.StackConfigurationFragment) string {
	image := r.DefaultStackHarnessImage
	if configuration.Image != nil {
		image = *configuration.Image
	}

	return fmt.Sprintf("%s:%s", image, configuration.Version)
}

func (r *StackReconciler) defaultJobContainerArgs(runID string) []string {
	return []string{
		fmt.Sprintf("--console-url=%s", r.ConsoleURL),
		fmt.Sprintf("--console-token=%s", r.DeployToken),
		fmt.Sprintf("--stack-run-id=%s", runID),
		fmt.Sprintf("--working-dir=%s", defaultJobVolumePath),
	}
}
