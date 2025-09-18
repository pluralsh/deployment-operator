package controller

import (
	"fmt"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podDefaultContainerAnnotation = "kubectl.kubernetes.io/default-container"
	defaultContainer              = "default"
	defaultVolumeName             = "default"
	defaultVolumePath             = "/plural"
	defaultTmpVolumeName          = "default-tmp"
	defaultTmpVolumePath          = "/tmp"
	nonRootUID                    = int64(65532)
	nonRootGID                    = nonRootUID
)

var (
	defaultContainerImages = map[console.AgentRuntimeType]string{
		console.AgentRuntimeTypeGemini: "ghcr.io/pluralsh/todo", // TODO
	}

	defaultContainerVersions = map[console.AgentRuntimeType]string{
		console.AgentRuntimeTypeGemini: "latest", // TODO
	}

	defaultVolume = corev1.Volume{
		Name: defaultVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	defaultContainerVolumeMount = corev1.VolumeMount{
		Name:      defaultVolumeName,
		MountPath: defaultVolumePath,
	}

	defaultTmpVolume = corev1.Volume{
		Name: defaultTmpVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	defaultTmpContainerVolumeMount = corev1.VolumeMount{
		Name:      defaultTmpVolumeName,
		MountPath: defaultTmpVolumePath,
	}
)

func buildAgentRunPod(run *v1alpha1.AgentRun, runtime *v1alpha1.AgentRuntime) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        run.Name,
			Namespace:   run.Namespace,
			Labels:      runtime.Spec.Template.Labels,
			Annotations: ensureAnnotations(runtime.Spec.Template.Annotations),
		},
		Spec: runtime.Spec.Template.Spec,
	}

	pod.Spec.Containers = ensureDefaultContainer(pod.Spec.Containers, run, runtime)
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.SecurityContext = ensureDefaultPodSecurityContext(pod.Spec.SecurityContext)
	pod.Spec.Volumes = ensureDefaultVolumes(pod.Spec.Volumes)

	//jobSpec.BackoffLimit = lo.ToPtr(int32(0)) TODO
	//jobSpec.TTLSecondsAfterFinished = lo.ToPtr(int32(60 * 60)) TODO

	return pod
}

func ensureAnnotations(annotations map[string]string) map[string]string {
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[podDefaultContainerAnnotation] = defaultContainer

	return annotations
}

func ensureDefaultContainer(containers []corev1.Container, run *v1alpha1.AgentRun, runtime *v1alpha1.AgentRuntime) []corev1.Container {
	if index := algorithms.Index(containers, func(container corev1.Container) bool {
		return container.Name == defaultContainer
	}); index == -1 {
		containers = append(containers, getDefaultContainer(run, runtime))
	} else {
		if containers[index].Image == "" {
			containers[index].Image = getDefaultContainerImage(containers[index].Image, runtime.Spec.Type)
		}

		containers[index].SecurityContext = ensureDefaultContainerSecurityContext(containers[index].SecurityContext)
		containers[index].EnvFrom = getDefaultContainerEnvFrom(run.Name)
		containers[index].VolumeMounts = ensureDefaultVolumeMounts(containers[index].VolumeMounts)
	}

	return containers
}

func ensureDefaultVolumeMounts(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	return append(
		algorithms.Filter(mounts, func(v corev1.VolumeMount) bool {
			return v.Name != defaultVolumeName && v.Name != defaultTmpVolumeName
		}),
		defaultContainerVolumeMount,
		defaultTmpContainerVolumeMount,
	)
}

func ensureDefaultVolumes(volumes []corev1.Volume) []corev1.Volume {
	return append(
		algorithms.Filter(volumes, func(v corev1.Volume) bool {
			return v.Name != defaultVolumeName && v.Name != defaultTmpVolumeName
		}),
		defaultVolume,
		defaultTmpVolume,
	)
}

func ensureDefaultPodSecurityContext(psc *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if psc != nil {
		return psc
	}

	return &corev1.PodSecurityContext{
		RunAsNonRoot: lo.ToPtr(true),
		RunAsUser:    lo.ToPtr(nonRootUID),
		RunAsGroup:   lo.ToPtr(nonRootGID),
	}
}

func getDefaultContainer(run *v1alpha1.AgentRun, runtime *v1alpha1.AgentRuntime) corev1.Container {
	return corev1.Container{
		Name:            defaultContainer,
		Image:           getDefaultContainerImage("", runtime.Spec.Type),
		VolumeMounts:    []corev1.VolumeMount{defaultContainerVolumeMount, defaultTmpContainerVolumeMount},
		SecurityContext: ensureDefaultContainerSecurityContext(nil),
		EnvFrom:         getDefaultContainerEnvFrom(run.Name),
	}
}

func getDefaultContainerImage(image string, agentRuntimeType console.AgentRuntimeType) string {
	if image != "" {
		return image
	}

	return fmt.Sprintf("%s:%s", defaultContainerImages[agentRuntimeType], defaultContainerVersions[agentRuntimeType])
}

func getDefaultContainerEnvFrom(secretName string) []corev1.EnvFromSource {
	return []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}},
	}}
}

func ensureDefaultContainerSecurityContext(sc *corev1.SecurityContext) *corev1.SecurityContext {
	if sc != nil {
		return sc
	}

	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: lo.ToPtr(false),
		ReadOnlyRootFilesystem:   lo.ToPtr(false),
		RunAsNonRoot:             lo.ToPtr(true),
		RunAsUser:                lo.ToPtr(nonRootUID),
		RunAsGroup:               lo.ToPtr(nonRootGID),
	}
}
