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

	dindContainerName            = "dind"
	defaultContainerDinDImage    = "docker"
	defaultContainerDinDImageTag = "27-dind"
	dockerCertsVolumeName        = "docker-certs"
	dockerGraphVolumeName        = "docker-graph"
	dockerCertsPath              = "/certs"
	dockerDaemonPort             = 2376
	dockerSocketGID              = int64(2375)

	dockerComposeVolumeName = "docker-compose"
	dockerComposeMountPath  = "/workspace/docker-compose.yaml"
)

var dindClientEnvs = []corev1.EnvVar{
	{Name: "DOCKER_HOST", Value: "tcp://localhost:2376"},
	{Name: "DOCKER_TLS_VERIFY", Value: "1"},
	{Name: "DOCKER_CERT_PATH", Value: dockerCertsPath + "/client"},
}

var (
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

	defaultContainerImage    = "ghcr.io/pluralsh/agent-harness"
	defaultContainerImageTag = "sha-cf549e2" // TODO make sure to change this for releases

	// Check .github/workflows/publish-agent-harness.yaml to see images being published.
	defaultContainerVersions = map[console.AgentRuntimeType]string{
		console.AgentRuntimeTypeClaude:   "%s-claude-1.0.128",
		console.AgentRuntimeTypeGemini:   "%s-gemini-0.6.1",
		console.AgentRuntimeTypeOpencode: "%s-opencode-0.15.4",
	}

	languages = map[console.AgentRunLanguage]string{
		console.AgentRunLanguageGo:         "golang",
		console.AgentRunLanguageJava:       "java",
		console.AgentRunLanguageJavascript: "node",
		console.AgentRunLanguagePython:     "python",
	}

	defaultVersions = map[console.AgentRunLanguage]string{
		console.AgentRunLanguageGo:         "1.24",
		console.AgentRunLanguageJava:       "25",
		console.AgentRunLanguageJavascript: "24",
		console.AgentRunLanguagePython:     "3.14",
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
	pod.Spec.Volumes = ensureDefaultVolumes(pod.Spec.Volumes)

	if runtime.Spec.HasDinDEnabled() {
		pod.Spec.SecurityContext = ensureDefaultPodSecurityContextWithDind(pod.Spec.SecurityContext)
		enableDind(pod)
	} else {
		pod.Spec.SecurityContext = ensureDefaultPodSecurityContext(pod.Spec.SecurityContext)
	}

	// Mount Docker Compose config if provided
	if runtime.Spec.DockerCompose != nil {
		addDockerComposeVolume(pod, run)
	}

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
			containers[index].Image = getDefaultContainerImage(containers[index].Image, runtime.Spec.Type, run.Spec.Language, run.Spec.LanguageVersion)
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
		Image:           getDefaultContainerImage("", runtime.Spec.Type, run.Spec.Language, run.Spec.LanguageVersion),
		VolumeMounts:    []corev1.VolumeMount{defaultTmpContainerVolumeMount},
		SecurityContext: ensureDefaultContainerSecurityContext(nil),
		EnvFrom:         getDefaultContainerEnvFrom(run.Name),
		Env:             getDefaultEnvVars(run),
	}
}

func getDefaultContainerImage(image string, agentRuntimeType console.AgentRuntimeType, language *console.AgentRunLanguage, version *string) string {
	if image != "" {
		return image
	}

	tag := fmt.Sprintf(defaultContainerVersions[agentRuntimeType], defaultContainerImageTag)

	// If the language name is recognized, append it to the tag along with the version (or default version).
	if lang, ok := languages[lo.FromPtr(language)]; ok {
		tag = fmt.Sprintf("%s-%s-%s", tag, lang,
			lo.Ternary(lo.IsEmpty(version), defaultVersions[lo.FromPtr(language)], lo.FromPtr(version)))
	}

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

func ensureDefaultPodSecurityContextWithDind(psc *corev1.PodSecurityContext) *corev1.PodSecurityContext {
	if psc != nil {
		// Add supplemental group if not already present
		if psc.SupplementalGroups == nil {
			psc.SupplementalGroups = []int64{}
		}

		// Check if docker group already exists
		hasDockerGroup := false
		for _, gid := range psc.SupplementalGroups {
			if gid == dockerSocketGID {
				hasDockerGroup = true
				break
			}
		}

		if !hasDockerGroup {
			psc.SupplementalGroups = append(psc.SupplementalGroups, dockerSocketGID)
		}

		return psc
	}

	// When DinD is enabled, don't set runAsNonRoot at pod level
	// because the DinD container needs to run as root
	return &corev1.PodSecurityContext{
		SupplementalGroups: []int64{dockerSocketGID},
	}
}

func enableDind(pod *corev1.Pod) {
	pod.Spec.Volumes = append(pod.Spec.Volumes,
		corev1.Volume{
			Name: dockerCertsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		corev1.Volume{
			Name: dockerGraphVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		// Add volume for Docker socket
		corev1.Volume{
			Name: "docker-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:  dindContainerName,
		Image: fmt.Sprintf("%s:%s", common.GetConfigurationManager().SwapBaseRegistry(defaultContainerDinDImage), defaultContainerDinDImageTag),
		SecurityContext: &corev1.SecurityContext{
			Privileged: lo.ToPtr(true),
		},
		Env: []corev1.EnvVar{
			{Name: "DOCKER_TLS_CERTDIR", Value: dockerCertsPath},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "docker",
				ContainerPort: dockerDaemonPort,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: dockerCertsVolumeName, MountPath: dockerCertsPath},
			{Name: dockerGraphVolumeName, MountPath: "/var/lib/docker"},
			// Mount the socket directory
			{Name: "docker-socket", MountPath: "/var/run"},
		},
	})

	// Wire agent container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == defaultContainer {
			c := &pod.Spec.Containers[i]

			c.Env = append(c.Env, dindClientEnvs...)

			c.VolumeMounts = append(c.VolumeMounts,
				corev1.VolumeMount{
					Name:      dockerCertsVolumeName,
					MountPath: dockerCertsPath,
					ReadOnly:  true,
				},
				// Mount the socket directory in the default container too
				corev1.VolumeMount{
					Name:      "docker-socket",
					MountPath: "/var/run",
					ReadOnly:  false,
				},
			)
		}
	}
}

func addDockerComposeVolume(pod *corev1.Pod, run *v1alpha1.AgentRun) {
	// Add volume referencing the ConfigMap containing Docker Compose config
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: dockerComposeVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: run.Name + "-docker-compose",
				},
			},
		},
	})

	// Mount the Docker Compose file in the default container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == defaultContainer {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      dockerComposeVolumeName,
					MountPath: dockerComposeMountPath,
					SubPath:   "docker-compose.yaml",
					ReadOnly:  true,
				},
			)
			break
		}
	}
}
