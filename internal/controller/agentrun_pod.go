package controller

import (
	"fmt"
	"os"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
)

const (
	podDefaultContainerAnnotation = "kubectl.kubernetes.io/default-container"
	defaultContainer              = "default"
	defaultTmpVolumeName          = "default-tmp"
	defaultTmpVolumePath          = "/tmp"
	nonRootUID                    = int64(65532)
	nonRootGID                    = nonRootUID
)

var (
	defaultContainerImage    = "ghcr.io/pluralsh/agent-harness"
	defaultContainerImageTag = "latest"

	defaultContainerVersions = map[console.AgentRuntimeType]string{
		console.AgentRuntimeTypeGemini:   "latest", // TODO
		console.AgentRuntimeTypeOpencode: "%s-opencode-0.15.4",
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

func init() {
	if os.Getenv("IMAGE_TAG") != "" {
		defaultContainerImageTag = os.Getenv("IMAGE_TAG")
	}
}

func buildAgentRunPod(run *v1alpha1.AgentRun, runtime *v1alpha1.AgentRuntime) *corev1.Pod {
	if runtime.Spec.Template == nil {
		runtime.Spec.Template = &corev1.PodTemplateSpec{}
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        run.Name,
			Namespace:   run.Namespace,
			Labels:      ensureDefaultLabels(runtime.Spec.Template.Labels, run),
			Annotations: ensureAnnotations(runtime.Spec.Template.Annotations),
		},
		Spec: runtime.Spec.Template.Spec,
	}

	pod.Spec.Containers = ensureDefaultContainer(pod.Spec.Containers, run, runtime)
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.SecurityContext = ensureDefaultPodSecurityContext(pod.Spec.SecurityContext)
	pod.Spec.Volumes = ensureDefaultVolumes(pod.Spec.Volumes)

	return pod
}

func ensureDefaultLabels(labels map[string]string, run *v1alpha1.AgentRun) map[string]string {
	if labels == nil {
		labels = map[string]string{}
	}

	// Add standard labels for agent runs
	labels["app.kubernetes.io/name"] = "agent-harness"
	labels["app.kubernetes.io/component"] = "agent-run"
	labels[v1alpha1.AgentRunIDLabel] = run.Status.GetID()

	return labels
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
		containers[index].Env = ensureDefaultEnvVars(containers[index].Env, run)

		// Do not allow command to be overridden. Only args can be overridden.
		containers[index].Command = nil
	}

	return containers
}

func ensureDefaultVolumeMounts(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	return append(
		algorithms.Filter(mounts, func(v corev1.VolumeMount) bool {
			return v.Name != defaultTmpVolumeName
		}),
		defaultTmpContainerVolumeMount,
	)
}

func ensureDefaultVolumes(volumes []corev1.Volume) []corev1.Volume {
	return append(
		algorithms.Filter(volumes, func(v corev1.Volume) bool {
			return v.Name != defaultTmpVolumeName
		}),
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
		VolumeMounts:    []corev1.VolumeMount{defaultTmpContainerVolumeMount},
		SecurityContext: ensureDefaultContainerSecurityContext(nil),
		EnvFrom:         getDefaultContainerEnvFrom(run.Name),
		Env:             getDefaultEnvVars(run),
	}
}

func getDefaultContainerImage(image string, agentRuntimeType console.AgentRuntimeType) string {
	if image != "" {
		return image
	}

	tag := fmt.Sprintf(defaultContainerVersions[agentRuntimeType], defaultContainerImageTag)

	return fmt.Sprintf("%s:%s", common.GetConfigurationManager().SwapBaseRegistry(defaultContainerImage), tag)
}

func getDefaultContainerEnvFrom(secretName string) []corev1.EnvFromSource {
	return []corev1.EnvFromSource{{
		SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: secretName}},
	}}
}

func getDefaultEnvVars(_ *v1alpha1.AgentRun) []corev1.EnvVar {
	return []corev1.EnvVar{}
}

func ensureDefaultEnvVars(existing []corev1.EnvVar, run *v1alpha1.AgentRun) []corev1.EnvVar {
	defaultEnvs := getDefaultEnvVars(run)

	// Add default env vars if they don't already exist
	for _, defaultEnv := range defaultEnvs {
		found := false
		for _, existingEnv := range existing {
			if existingEnv.Name == defaultEnv.Name {
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, defaultEnv)
		}
	}

	return existing
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
