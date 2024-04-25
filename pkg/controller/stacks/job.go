package stacks

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/polly/algorithms"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	jobSelector          = "stackrun.deployments.plural.sh"
	DefaultJobContainer  = "default"
	defaultJobVolume     = "default"
	defaultJobVolumePath = "/harness"
)

func (r *StackReconciler) GenerateRunJob(run *console.StackRunFragment, name string) *batchv1.Job {
	var jobSpec *batchv1.JobSpec

	// Use job spec defined in run as base if it is available.
	if run.JobSpec != nil {
		jobSpec = getRunJobSpec(name, run.JobSpec)
	}

	// If user-defined job spec was not available initialize it here.
	if jobSpec == nil {
		jobSpec = &batchv1.JobSpec{}
	}

	// Set requirements like name, namespace, container and volume.
	jobSpec.Template.ObjectMeta.Name = name

	if jobSpec.Template.ObjectMeta.Namespace == "" {
		jobSpec.Template.ObjectMeta.Namespace = r.Namespace
	}

	r.ensureDefaultContainer(jobSpec.Template.Spec.Containers, run)

	ensureDefaultVolume(jobSpec.Template.Spec.Volumes)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   r.Namespace,
			Annotations: map[string]string{jobSelector: name},
			Labels:      map[string]string{jobSelector: name},
		},
		Spec: *jobSpec,
	}
}

func getRunJobSpec(name string, jobSpecFragment *console.JobSpecFragment) *batchv1.JobSpec {
	if jobSpecFragment == nil {
		return nil
	}

	var jobSpec *batchv1.JobSpec
	if jobSpecFragment.Raw != nil && *jobSpecFragment.Raw != "null" {
		job, err := consoleclient.JobFromYaml(*jobSpecFragment.Raw)
		if err != nil {
			return nil
		}

		jobSpec = &job.Spec
	} else {
		jobSpec = &batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   jobSpecFragment.Namespace,
					Labels:      consoleclient.StringMapFromInterfaceMap(jobSpecFragment.Labels),
					Annotations: consoleclient.StringMapFromInterfaceMap(jobSpecFragment.Annotations),
				},
				Spec: corev1.PodSpec{
					Containers: consoleclient.ContainersFromContainerSpecFragments(name, jobSpecFragment.Containers),
				},
			},
		}
	}

	return jobSpec
}

func (r *StackReconciler) ensureDefaultContainer(containers []corev1.Container, run *console.StackRunFragment) {
	if index := algorithms.Index(containers, func(container corev1.Container) bool {
		return container.Name == DefaultJobContainer
	}); index == -1 {
		containers = append(containers, r.getDefaultContainer(run))
	} else {
		if containers[index].Image == "" {
			containers[index].Image = r.getDefaultContainerImage(run.Configuration)
		}

		containers[index].Args = r.getDefaultContainerArgs(run.ID)

		ensureDefaultVolumeMount(containers[index].VolumeMounts)
	}
}

func (r *StackReconciler) getDefaultContainer(run *console.StackRunFragment) corev1.Container {
	return corev1.Container{
		Name:         DefaultJobContainer,
		Image:        r.getDefaultContainerImage(run.Configuration),
		Args:         r.getDefaultContainerArgs(run.ID),
		VolumeMounts: []corev1.VolumeMount{getDefaultContainerVolumeMount()},
	}
}

func (r *StackReconciler) getDefaultContainerImage(configuration *console.StackConfigurationFragment) string {
	image := r.DefaultStackHarnessImage
	if configuration.Image != nil {
		image = *configuration.Image
	}

	return fmt.Sprintf("%s:%s", image, configuration.Version)
}

func (r *StackReconciler) getDefaultContainerArgs(runID string) []string {
	return []string{
		fmt.Sprintf("--console-url=%s", r.ConsoleURL),
		fmt.Sprintf("--console-token=%s", r.DeployToken),
		fmt.Sprintf("--stack-run-id=%s", runID),
	}
}

func getDefaultContainerVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      defaultJobVolume,
		MountPath: defaultJobVolumePath,
	}
}

func ensureDefaultVolumeMount(mounts []corev1.VolumeMount) {
	if index := algorithms.Index(mounts, func(mount corev1.VolumeMount) bool {
		return mount.Name == defaultJobVolume
	}); index == -1 {
		mounts = append(mounts, getDefaultContainerVolumeMount())
	} else {
		mounts[index] = getDefaultContainerVolumeMount()
	}
}

func ensureDefaultVolume(volumes []corev1.Volume) {
	if index := algorithms.Index(volumes, func(volume corev1.Volume) bool {
		return volume.Name == defaultJobVolume
	}); index == -1 {
		volumes = append(volumes, getDefaultVolume())
	} else {
		volumes[index] = getDefaultVolume()
	}
}

func getDefaultVolume() corev1.Volume {
	return corev1.Volume{
		Name: defaultJobVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}
