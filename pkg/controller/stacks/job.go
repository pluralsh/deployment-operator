package stacks

import (
	"context"
	"fmt"
	"os"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/pluralsh/deployment-operator/internal/metrics"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
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
	jobName := GetRunJobName(run)
	foundJob := &batchv1.Job{}
	if err := r.K8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: r.Namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		logger.V(2).Info("generating job", "namespace", r.Namespace, "name", jobName)
		job := r.GenerateRunJob(run, jobName)

		logger.V(2).Info("creating job", "namespace", job.Namespace, "name", job.Name)
		if err := r.K8sClient.Create(ctx, job); err != nil {
			logger.Error(err, "unable to create job")
			return nil, err
		}

		metrics.Record().StackRunJobCreation()
		if err := r.ConsoleClient.UpdateStackRun(run.ID, console.StackRunAttributes{
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

func GetRunJobName(run *console.StackRunFragment) string {
	return fmt.Sprintf("stack-%s", run.ID)
}

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

	if jobSpec.Template.Annotations == nil {
		jobSpec.Template.Annotations = map[string]string{}
	}
	jobSpec.Template.Annotations[podDefaultContainerAnnotation] = DefaultJobContainer

	if jobSpec.Template.ObjectMeta.Namespace == "" {
		jobSpec.Template.ObjectMeta.Namespace = r.Namespace
	}

	jobSpec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	jobSpec.BackoffLimit = lo.ToPtr(int32(0))
	jobSpec.TTLSecondsAfterFinished = lo.ToPtr(int32(60 * 60))

	jobSpec.Template.Spec.Containers = r.ensureDefaultContainer(jobSpec.Template.Spec.Containers, run)

	jobSpec.Template.Spec.Volumes = ensureDefaultVolumes(jobSpec.Template.Spec.Volumes)

	jobSpec.Template.Spec.SecurityContext = ensureDefaultPodSecurityContext(jobSpec.Template.Spec.SecurityContext)

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
	}
}

func (r *StackReconciler) getDefaultContainerImage(run *console.StackRunFragment) string {
	image := defaultContainerImages[run.Type]
	version := defaultContainerVersions[run.Type]
	if run.Configuration.Version != "" {
		version = run.Configuration.Version
	}

	if run.Configuration.Image != nil && *run.Configuration.Image != "" {
		image = *run.Configuration.Image
		return fmt.Sprintf("%s:%s", image, version)
	}

	return fmt.Sprintf("%s:%s-%s-%s", image, defaultImageTag, strings.ToLower(string(run.Type)), version)
}

func (r *StackReconciler) getDefaultContainerArgs(runID string) []string {
	return []string{
		fmt.Sprintf("--console-url=%s", r.ConsoleURL),
		fmt.Sprintf("--console-token=%s", r.DeployToken),
		fmt.Sprintf("--stack-run-id=%s", runID),
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
