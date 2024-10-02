package stacks

import (
	"context"
	"fmt"
	"os"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/metrics"
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
	jobSelector                   = "stackrun.deployments.plural.sh"
	DefaultJobContainer           = "default"
	defaultJobVolumeName          = "default"
	defaultJobVolumePath          = "/plural"
	defaultJobTmpVolumeName       = "default-tmp"
	defaultJobTmpVolumePath       = "/tmp"
	nonRootUID                    = int64(65532)
	nonRootGID                    = nonRootUID
)

var (
	defaultContainerImages = map[console.StackType]string{
		console.StackTypeTerraform: "ghcr.io/pluralsh/harness",
		console.StackTypeAnsible:   "ghcr.io/pluralsh/harness",
	}

	defaultContainerVersions = map[console.StackType]string{
		console.StackTypeTerraform: "1.8.2",
		console.StackTypeAnsible:   "latest",
	}

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

	defaultImageTag = "0.4.29"
)

func init() {
	if os.Getenv("IMAGE_TAG") != "" {
		defaultImageTag = os.Getenv("IMAGE_TAG")
	}
}

func (r *StackReconciler) reconcileRunJob(ctx context.Context, run *console.StackRunFragment) (*batchv1.Job, error) {
	logger := log.FromContext(ctx)

	name := GetRunResourceName(run)
	jobSpec := getRunJobSpec(name, run.JobSpec)
	namespace := r.GetRunResourceNamespace(jobSpec)

	foundJob := &batchv1.Job{}
	if err := r.k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		if _, err = r.upsertRunSecret(ctx, name, namespace); err != nil {
			return nil, err
		}

		job := r.GenerateRunJob(run, jobSpec, name, namespace)
		logger.V(2).Info("creating job for stack run", "id", run.ID, "namespace", job.Namespace, "name", job.Name)
		if err := r.k8sClient.Create(ctx, job); err != nil {
			logger.Error(err, "unable to create job")
			return nil, err
		}

		metrics.Record().StackRunJobCreation()
		if err := r.consoleClient.UpdateStackRun(run.ID, console.StackRunAttributes{
			Status: run.Status,
			JobRef: &console.NamespacedName{
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
func GetRunResourceName(run *console.StackRunFragment) string {
	return fmt.Sprintf("stack-%s", run.ID)
}

// GetRunResourceNamespace returns a resource namespace used for a job and a secret connected to a given run.
func (r *StackReconciler) GetRunResourceNamespace(jobSpec *batchv1.JobSpec) (namespace string) {
	if jobSpec != nil {
		namespace = jobSpec.Template.Namespace
	}

	if namespace == "" {
		namespace = r.namespace
	}

	return
}

func (r *StackReconciler) GenerateRunJob(run *console.StackRunFragment, jobSpec *batchv1.JobSpec, name, namespace string) *batchv1.Job {
	// If user-defined job spec was not available initialize it here.
	if jobSpec == nil {
		jobSpec = &batchv1.JobSpec{}
	}

	// Set requirements like name, namespace, container and volume.
	jobSpec.Template.ObjectMeta.Name = name
	jobSpec.Template.ObjectMeta.Namespace = namespace

	if jobSpec.Template.Annotations == nil {
		jobSpec.Template.Annotations = map[string]string{}
	}
	jobSpec.Template.Annotations[podDefaultContainerAnnotation] = DefaultJobContainer

	jobSpec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	jobSpec.BackoffLimit = lo.ToPtr(int32(0))
	jobSpec.TTLSecondsAfterFinished = lo.ToPtr(int32(60 * 60))

	jobSpec.Template.Spec.Containers = r.ensureDefaultContainer(jobSpec.Template.Spec.Containers, run)

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
	}
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
					Containers: consoleclient.ContainersFromContainerSpecFragments(name, jobSpecFragment.Containers),
				},
			},
		}

		if jobSpecFragment.ServiceAccount != nil {
			jobSpec.Template.Spec.ServiceAccountName = *jobSpecFragment.ServiceAccount
		}
	}

	return jobSpec
}

func (r *StackReconciler) ensureDefaultContainer(containers []corev1.Container, run *console.StackRunFragment) []corev1.Container {
	if index := algorithms.Index(containers, func(container corev1.Container) bool {
		return container.Name == DefaultJobContainer
	}); index == -1 {
		containers = append(containers, r.getDefaultContainer(run))
	} else {
		if containers[index].Image == "" {
			containers[index].Image = r.getDefaultContainerImage(run)
		}

		containers[index].Args = r.getDefaultContainerArgs(run.ID)

		containers[index].EnvFrom = r.getDefaultContainerEnvFrom(run)

		containers[index].VolumeMounts = ensureDefaultVolumeMounts(containers[index].VolumeMounts)
	}
	return containers
}

func (r *StackReconciler) getDefaultContainer(run *console.StackRunFragment) corev1.Container {
	return corev1.Container{
		Name:  DefaultJobContainer,
		Image: r.getDefaultContainerImage(run),
		Args:  r.getDefaultContainerArgs(run.ID),
		VolumeMounts: []corev1.VolumeMount{
			defaultJobContainerVolumeMount,
			defaultJobTmpContainerVolumeMount,
		},
		SecurityContext: ensureDefaultContainerSecurityContext(nil),
		Env:             make([]corev1.EnvVar, 0),
		EnvFrom:         r.getDefaultContainerEnvFrom(run),
	}
}

func (r *StackReconciler) getDefaultContainerImage(run *console.StackRunFragment) string {
	// In case image is not provided, it will use our default image.
	// Image name format: <defaultImage>:<tag>
	// Note: User has to make sure that the tag is correct and matches our naming scheme.
	//
	// In case image is provided, it will replace both image and tag with provided values.
	// Image name format: <image>:<tag>
	if r.hasCustomTag(run) {
		return fmt.Sprintf("%s:%s", r.getImage(run), *run.Configuration.Tag)
	}

	// In case there is a custom version and a custom image provided, it will replace both image and version
	// with provided values.
	// Image name format: <image>:<version>
	if r.hasCustomImage(run) && r.hasCustomVersion(run) {
		return fmt.Sprintf("%s:%s", *run.Configuration.Image, *run.Configuration.Version)
	}

	// In case only image is provided, do not follow our default naming scheme.
	// Image name format: <image>:<defaultTag>
	// Note: User has to make sure that the image contains the tag matching our defaults.
	if r.hasCustomImage(run) {
		return fmt.Sprintf("%s:%s", *run.Configuration.Image, r.getTag(run))
	}

	// In any other case return image in the default format: <defaultImage>:<defaultTag>-<stackType>-<defaultVersionOrVersion>.
	// In this case only a custom tool version is ever provided to override our defaults.
	return fmt.Sprintf("%s:%s-%s-%s", r.getImage(run), r.getTag(run), strings.ToLower(string(run.Type)), r.getVersion(run))
}

func (r *StackReconciler) hasCustomImage(run *console.StackRunFragment) bool {
	return run.Configuration.Image != nil && len(*run.Configuration.Image) > 0
}

func (r *StackReconciler) getImage(run *console.StackRunFragment) string {
	if r.hasCustomImage(run) {
		return *run.Configuration.Image
	}

	return defaultContainerImages[run.Type]
}

func (r *StackReconciler) hasCustomVersion(run *console.StackRunFragment) bool {
	return run.Configuration.Version != nil && len(*run.Configuration.Version) > 0
}

func (r *StackReconciler) getVersion(run *console.StackRunFragment) string {
	if r.hasCustomVersion(run) {
		return *run.Configuration.Version
	}

	return defaultContainerVersions[run.Type]
}

func (r *StackReconciler) hasCustomTag(run *console.StackRunFragment) bool {
	return run.Configuration.Tag != nil && len(*run.Configuration.Tag) > 0
}

func (r *StackReconciler) getTag(run *console.StackRunFragment) string {
	if r.hasCustomTag(run) {
		return *run.Configuration.Tag
	}

	return defaultImageTag
}

func (r *StackReconciler) getDefaultContainerEnvFrom(run *console.StackRunFragment) []corev1.EnvFromSource {
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

func (r *StackReconciler) getDefaultContainerArgs(runID string) []string {
	return []string{fmt.Sprintf("--stack-run-id=%s", runID)}
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
