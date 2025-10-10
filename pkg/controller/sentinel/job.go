package stacks

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	podDefaultContainerAnnotation = "kubectl.kubernetes.io/default-container"
	jobSelector                   = "sentinelrun.deployments.plural.sh"
	DefaultJobContainer           = "default"
	defaultJobVolumeName          = "default"
	defaultJobVolumePath          = "/plural"
	defaultJobTmpVolumeName       = "default-tmp"
	defaultJobTmpVolumePath       = "/tmp"
	nonRootUID                    = int64(65532)
	nonRootGID                    = nonRootUID
	defaultContainerImage         = "ghcr.io/pluralsh/harness-sentinel:latest"
)

var (
	defaultJobVolume = corev1.Volume{
		Name: defaultJobVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	defaultJobContainerVolumeMount = corev1.VolumeMount{
		Name:      defaultJobVolumeName,
		MountPath: defaultJobVolumePath,
	}

	defaultJobTmpVolume = corev1.Volume{
		Name: defaultJobTmpVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	defaultJobTmpContainerVolumeMount = corev1.VolumeMount{
		Name:      defaultJobTmpVolumeName,
		MountPath: defaultJobTmpVolumePath,
	}
)

func (r *SentinelReconciler) reconcileRunJob(ctx context.Context, run *console.SentinelRunJobFragment) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)

	name := GetRunResourceName(run)
	jobSpec := getRunJobSpec(name, run.Job)
	namespace := r.GetRunResourceNamespace(jobSpec)

	foundJob := &batchv1.Job{}
	if err := r.k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		secret, err := r.upsertRunSecret(ctx, name, namespace, run.ID)
		if err != nil {
			return nil, err
		}

		job, err := r.GenerateRunJob(run, jobSpec, name, namespace)
		if err != nil {
			return nil, err
		}
		logger.V(2).Info("creating job for stack run", "id", run.ID, "namespace", job.Namespace, "name", job.Name)
		if err := r.k8sClient.Create(ctx, job); err != nil {
			logger.Error(err, "unable to create job")
			return nil, err
		}

		if err := utils.TryAddOwnerRef(ctx, r.k8sClient, job, secret, r.scheme); err != nil {
			logger.Error(err, "error setting owner reference for job secret")
			return nil, err
		}

		metrics.Record().StackRunJobCreation()
		if _, err := r.consoleClient.UpdateSentinelRunJobStatus(run.ID, &console.SentinelRunJobUpdateAttributes{
			Status: lo.ToPtr(run.Status),
			Reference: &console.NamespacedName{
				Name:      job.Name,
				Namespace: job.Namespace,
			},
		}); err != nil {
			return nil, err
		}

		return job, nil
	}

	return foundJob, nil
}

// GetRunResourceName returns a resource name used for a job and a secret connected to a given run.
func GetRunResourceName(run *console.SentinelRunJobFragment) string {
	return fmt.Sprintf("sentinel-%s", run.ID)
}

// GetRunResourceNamespace returns a resource namespace used for a job and a secret connected to a given run.
func (r *SentinelReconciler) GetRunResourceNamespace(jobSpec *batchv1.JobSpec) (namespace string) {
	if jobSpec != nil {
		namespace = jobSpec.Template.Namespace
	}

	if namespace == "" {
		namespace = r.namespace
	}

	return
}

func (r *SentinelReconciler) GenerateRunJob(run *console.SentinelRunJobFragment, jobSpec *batchv1.JobSpec, name, namespace string) (*batchv1.Job, error) {
	var err error
	// If user-defined job spec was not available initialize it here.
	if jobSpec == nil {
		jobSpec = &batchv1.JobSpec{}
	}

	// Set requirements like name, namespace, container and volume.
	jobSpec.Template.Name = name
	jobSpec.Template.Namespace = namespace

	if jobSpec.Template.Annotations == nil {
		jobSpec.Template.Annotations = map[string]string{}
	}
	jobSpec.Template.Annotations[podDefaultContainerAnnotation] = DefaultJobContainer

	jobSpec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	jobSpec.BackoffLimit = lo.ToPtr(int32(0))
	jobSpec.TTLSecondsAfterFinished = lo.ToPtr(int32(60 * 60))

	jobSpec.Template.Spec.Containers = r.ensureDefaultContainer(jobSpec.Template.Spec.Containers, run)

	jobSpec.Template.Spec.Containers, err = r.ensureDefaultContainerResourcesRequests(jobSpec.Template.Spec.Containers, run)
	if err != nil {
		return nil, err
	}

	jobSpec.Template.Spec.Volumes = ensureDefaultVolumes(jobSpec.Template.Spec.Volumes)

	jobSpec.Template.Spec.SecurityContext = ensureDefaultPodSecurityContext(jobSpec.Template.Spec.SecurityContext)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{jobSelector: name},
			Labels:      map[string]string{jobSelector: name},
		},
		Spec: *jobSpec,
	}, nil
}

func getRunJobSpec(name string, jobSpecFragment *console.JobSpecFragment) *batchv1.JobSpec {
	if jobSpecFragment == nil {
		return nil
	}
	var jobSpec *batchv1.JobSpec
	var err error
	if jobSpecFragment.Raw != nil && *jobSpecFragment.Raw != "null" {
		jobSpec, err = consoleclient.JobSpecFromYaml(*jobSpecFragment.Raw)
		if err != nil {
			return nil
		}
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
					Containers: consoleclient.ContainersFromContainerSpecFragments(name, jobSpecFragment.Containers, jobSpecFragment.Requests),
				},
			},
		}

		if jobSpecFragment.ServiceAccount != nil {
			jobSpec.Template.Spec.ServiceAccountName = *jobSpecFragment.ServiceAccount
		}
	}

	return jobSpec
}

func (r *SentinelReconciler) ensureDefaultContainer(
	containers []corev1.Container,
	run *console.SentinelRunJobFragment,
) []corev1.Container {

	// If user specified containers, don't infer anything
	if len(containers) > 0 {
		// optionally normalize the default container (if they used the default name)
		index := algorithms.Index(containers, func(c corev1.Container) bool {
			return c.Name == DefaultJobContainer
		})

		if index != -1 {
			// Only patch minimal defaults, don’t override user intent
			if containers[index].Image == "" {
				containers[index].Image = defaultContainerImage
			}
			containers[index].VolumeMounts = ensureDefaultVolumeMounts(containers[index].VolumeMounts)
		}

		return containers
	}

	// If no containers at all, inject a default one (for safety)
	return []corev1.Container{r.getDefaultContainer(run)}
}

func (r *SentinelReconciler) getDefaultContainer(run *console.SentinelRunJobFragment) corev1.Container {
	return corev1.Container{
		Name:  DefaultJobContainer,
		Image: defaultContainerImage,
		VolumeMounts: []corev1.VolumeMount{
			defaultJobContainerVolumeMount,
			defaultJobTmpContainerVolumeMount,
		},
		SecurityContext: ensureDefaultContainerSecurityContext(nil),
		Env:             make([]corev1.EnvVar, 0),
		EnvFrom:         r.getDefaultContainerEnvFrom(run),
	}
}

func (r *SentinelReconciler) getDefaultContainerEnvFrom(run *console.SentinelRunJobFragment) []corev1.EnvFromSource {
	return []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: GetRunResourceName(run),
				},
			},
		},
	}
}

func ensureDefaultVolumeMounts(mounts []corev1.VolumeMount) []corev1.VolumeMount {
	return append(
		algorithms.Filter(mounts, func(v corev1.VolumeMount) bool {
			switch v.Name {
			case defaultJobVolumeName:
			case defaultJobTmpVolumeName:
				return false
			}

			return true
		}),
		defaultJobContainerVolumeMount,
		defaultJobTmpContainerVolumeMount,
	)
}

func ensureDefaultVolumes(volumes []corev1.Volume) []corev1.Volume {
	return append(
		algorithms.Filter(volumes, func(v corev1.Volume) bool {
			switch v.Name {
			case defaultJobVolumeName:
			case defaultJobTmpVolumeName:
				return false
			}

			return true
		}),
		defaultJobVolume,
		defaultJobTmpVolume,
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

func (r *SentinelReconciler) ensureDefaultContainerResourcesRequests(containers []corev1.Container, run *console.SentinelRunJobFragment) ([]corev1.Container, error) {
	if run.Job == nil || run.Job.Requests == nil {
		return containers, nil
	}
	if run.Job.Requests.Requests == nil && run.Job.Requests.Limits == nil {
		return containers, nil
	}

	for i, container := range containers {
		if run.Job.Requests.Requests != nil {
			if len(container.Resources.Requests) == 0 {
				containers[i].Resources.Requests = map[corev1.ResourceName]resource.Quantity{}
			}
			if run.Job.Requests.Requests.CPU != nil {
				cpu, err := resource.ParseQuantity(*run.Job.Requests.Requests.CPU)
				if err != nil {
					return nil, err
				}
				containers[i].Resources.Requests[corev1.ResourceCPU] = cpu
			}
			if run.Job.Requests.Requests.Memory != nil {
				memory, err := resource.ParseQuantity(*run.Job.Requests.Requests.Memory)
				if err != nil {
					return nil, err
				}
				containers[i].Resources.Requests[corev1.ResourceMemory] = memory
			}
		}
		if run.Job.Requests.Limits != nil {
			if len(container.Resources.Limits) == 0 {
				containers[i].Resources.Limits = map[corev1.ResourceName]resource.Quantity{}
			}
			if run.Job.Requests.Limits.CPU != nil {
				cpu, err := resource.ParseQuantity(*run.Job.Requests.Limits.CPU)
				if err != nil {
					return nil, err
				}
				containers[i].Resources.Limits[corev1.ResourceCPU] = cpu
			}
			if run.Job.Requests.Limits.Memory != nil {
				memory, err := resource.ParseQuantity(*run.Job.Requests.Limits.Memory)
				if err != nil {
					return nil, err
				}
				containers[i].Resources.Limits[corev1.ResourceMemory] = memory
			}
		}
	}

	return containers, nil
}
