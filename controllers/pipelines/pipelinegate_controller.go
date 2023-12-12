/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pipelines

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/apis/pipelines/v1alpha1"
	console "github.com/pluralsh/deployment-operator/client"

	//job "k8s.io/api/batch/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineGateReconciler reconciles a PipelineGate object
type PipelineGateReconciler struct {
	client.Client
	consoleClient console.Client
	Scheme        *runtime.Scheme
	Log           logr.Logger
}

//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=pipelines.plural.sh,resources=pipelinegates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pipelinegate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *PipelineGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	log := r.Log.WithValues("workspace", req.NamespacedName)

	pipelineGateInstance := &pipelinesv1alpha1.PipelineGate{}

	// get pipelinegate
	if err := r.Get(ctx, req.NamespacedName, pipelineGateInstance); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Unable to fetch PipelineGate - skipping", "namespace", pipelineGateInstance.Namespace, "name", pipelineGateInstance.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PipelineGate")
		return ctrl.Result{}, err
	}

	// if pipelinegate is terminating, no need to reconcile
	if pipelineGateInstance.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if pipelineGateInstance.Status.State == console.GateStateClosed {
		return ctrl.Result{}, nil
	}

	// create job
	job := r.generateJob(ctx, log, pipelineGateInstance)
	if err := ctrl.SetControllerReference(pipelineGateInstance, job, r.Scheme); err != nil {
		log.Error(err, "Error setting ControllerReference for Job")
		return ctrl.Result{}, err
	}
	if err := Job(ctx, r.Client, job, log); err != nil {
		log.Error(err, "Error reconciling Job", "job", job.Name, "namespace", job.Namespace)
		return ctrl.Result{}, err
	}

	// update pipelinegate status
	// update job status
	// include a ttl?

	return ctrl.Result{}, nil
}

//func (r *PipelineGateReconciler) updateStatusAndReturn(ctx context.Context, pipelineGateInstance *pipelinesv1alpha1.PipelineGate, gateState pipelinesv1alpha1.GateState, message string) (ctrl.Result, error) {
//	pipelineGateInstance.Status.GateState = gateState
//	if err := r.Update(ctx, pipelineGateInstance); err != nil {
//		return ctrl.Result{}, err
//	}
//	return ctrl.Result{}, nil
//}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelinesv1alpha1.PipelineGate{}).
		//Owns(&corev1.Namespace{}).
		//Owns(&corev1.ServiceAccount{}).
		//Owns(&rbacv1.RoleBinding{}).
		Complete(r)

}

// Job reconciles a k8s job object.
func Job(ctx context.Context, r client.Client, job *batchv1.Job, log logr.Logger) error {
	foundJob := &batchv1.Job{}
	justCreated := false
	if err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating Job", "namespace", job.Namespace, "name", job.Name)
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Unable to create Job")
				return err
			}
			justCreated = true
		} else {
			log.Error(err, "Error getting Job")
			return err
		}
	}
	if !justCreated && CopyJobFields(job, foundJob, log) {
		log.Info("Updating Job", "namespace", job.Namespace, "name", job.Name)
		if err := r.Update(ctx, foundJob); err != nil {
			log.Error(err, "Unable to update Job")
			return err
		}
	}

	return nil
}

func (r *PipelineGateReconciler) generateJob(ctx context.Context, log logr.Logger, pipelineGateInstance *pipelinesv1alpha1.PipelineGate) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineGateInstance.Name,
			Namespace: pipelineGateInstance.Namespace,
		},
		Spec: pipelineGateInstance.Spec.GateSpec.JobSpec,
	}
}

func CopyJobFields(from, to *batchv1.Job, log logr.Logger) bool {
	requireUpdate := false
	for k, v := range to.Labels {
		if from.Labels[k] != v {
			log.V(1).Info("reconciling Job due to label change")
			log.V(2).Info("difference in Job labels", "wanted", from.Labels, "existing", to.Labels)
			requireUpdate = true
		}
	}
	if len(to.Labels) == 0 && len(from.Labels) != 0 {
		log.V(1).Info("reconciling Job due to label change")
		log.V(2).Info("difference in Job labels", "wanted", from.Labels, "existing", to.Labels)
		requireUpdate = true
	}
	to.Labels = from.Labels

	// for k, v := range to.Annotations {
	// 	if from.Annotations[k] != v {
	// 		log.V(1).Info("reconciling Job due to annotation change")
	// 		log.V(2).Info("difference in Job annotations", "wanted", from.Annotations, "existing", to.Annotations)
	// 		requireUpdate = true
	// 	}
	// }
	// if len(to.Annotations) == 0 && len(from.Annotations) != 0 {
	// 	log.V(1).Info("reconciling Job due to annotation change")
	// 	log.V(2).Info("difference in Job annotations", "wanted", from.Annotations, "existing", to.Annotations)
	// 	requireUpdate = true
	// }
	// to.Annotations = from.Annotations

	for k, v := range to.Spec.Template.Labels {
		if from.Spec.Template.Labels[k] != v {
			log.V(1).Info("reconciling Job due to template label change")
			log.V(2).Info("difference in Job template labels", "wanted", from.Spec.Template.Labels, "existing", to.Spec.Template.Labels)
			requireUpdate = true
		}
	}
	if len(to.Spec.Template.Labels) == 0 && len(from.Spec.Template.Labels) != 0 {
		log.V(1).Info("reconciling Job due to template label change")
		log.V(2).Info("difference in Job template labels", "wanted", from.Spec.Template.Labels, "existing", to.Spec.Template.Labels)
		requireUpdate = true
	}
	to.Spec.Template.Labels = from.Spec.Template.Labels

	for k, v := range to.Spec.Template.Annotations {
		if from.Spec.Template.Annotations[k] != v {
			log.V(1).Info("reconciling Job due to template annotation change")
			log.V(2).Info("difference in Job template annotations", "wanted", from.Spec.Template.Annotations, "existing", to.Spec.Template.Annotations)
			requireUpdate = true
		}
	}
	if len(to.Spec.Template.Annotations) == 0 && len(from.Spec.Template.Annotations) != 0 {
		log.V(1).Info("reconciling Job due to template annotation change")
		log.V(2).Info("difference in Job template annotations", "wanted", from.Spec.Template.Annotations, "existing", to.Spec.Template.Annotations)
		requireUpdate = true
	}
	to.Spec.Template.Annotations = from.Spec.Template.Annotations

	if !reflect.DeepEqual(to.Spec.Template.Spec.Volumes, from.Spec.Template.Spec.Volumes) {
		log.V(1).Info("reconciling Job due to volumes change")
		log.V(2).Info("difference in Job volumes", "wanted", from.Spec.Template.Spec.Volumes, "existing", to.Spec.Template.Spec.Volumes)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Volumes = from.Spec.Template.Spec.Volumes

	if !reflect.DeepEqual(to.Spec.Template.Spec.ServiceAccountName, from.Spec.Template.Spec.ServiceAccountName) {
		log.V(1).Info("reconciling Job due to service account name change")
		log.V(2).Info("difference in Job service account name", "wanted", from.Spec.Template.Spec.ServiceAccountName, "existing", to.Spec.Template.Spec.ServiceAccountName)
		requireUpdate = true
	}
	to.Spec.Template.Spec.ServiceAccountName = from.Spec.Template.Spec.ServiceAccountName

	if !reflect.DeepEqual(to.Spec.Template.Spec.SecurityContext, from.Spec.Template.Spec.SecurityContext) {
		log.V(1).Info("reconciling Job due to security context change")
		log.V(2).Info("difference in Job security context", "wanted", from.Spec.Template.Spec.SecurityContext, "existing", to.Spec.Template.Spec.SecurityContext)
		requireUpdate = true
	}
	to.Spec.Template.Spec.SecurityContext = from.Spec.Template.Spec.SecurityContext

	if !reflect.DeepEqual(to.Spec.Template.Spec.Affinity, from.Spec.Template.Spec.Affinity) {
		log.V(1).Info("reconciling Job due to affinity change")
		log.V(2).Info("difference in Job affinity", "wanted", from.Spec.Template.Spec.Affinity, "existing", to.Spec.Template.Spec.Affinity)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Affinity = from.Spec.Template.Spec.Affinity

	if !reflect.DeepEqual(to.Spec.Template.Spec.Tolerations, from.Spec.Template.Spec.Tolerations) {
		log.V(1).Info("reconciling Job due to toleration change")
		log.V(2).Info("difference in Job tolerations", "wanted", from.Spec.Template.Spec.Tolerations, "existing", to.Spec.Template.Spec.Tolerations)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Tolerations = from.Spec.Template.Spec.Tolerations

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].Name, from.Spec.Template.Spec.Containers[0].Name) {
		log.V(1).Info("reconciling Job due to container[0] name change")
		log.V(2).Info("difference in Job container[0] name", "wanted", from.Spec.Template.Spec.Containers[0].Name, "existing", to.Spec.Template.Spec.Containers[0].Name)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].Name = from.Spec.Template.Spec.Containers[0].Name

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].Image, from.Spec.Template.Spec.Containers[0].Image) {
		log.V(1).Info("reconciling Job due to container[0] image change")
		log.V(2).Info("difference in Job container[0] image", "wanted", from.Spec.Template.Spec.Containers[0].Image, "existing", to.Spec.Template.Spec.Containers[0].Image)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].Image = from.Spec.Template.Spec.Containers[0].Image

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].WorkingDir, from.Spec.Template.Spec.Containers[0].WorkingDir) {
		log.V(1).Info("reconciling Job due to container[0] working dir change")
		log.V(2).Info("difference in Job container[0] working dir", "wanted", from.Spec.Template.Spec.Containers[0].WorkingDir, "existing", to.Spec.Template.Spec.Containers[0].WorkingDir)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].WorkingDir = from.Spec.Template.Spec.Containers[0].WorkingDir

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].Ports, from.Spec.Template.Spec.Containers[0].Ports) {
		log.V(1).Info("reconciling Job due to container[0] port change")
		log.V(2).Info("difference in Job container[0] ports", "wanted", from.Spec.Template.Spec.Containers[0].Ports, "existing", to.Spec.Template.Spec.Containers[0].Ports)

		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].Ports = from.Spec.Template.Spec.Containers[0].Ports

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].Env, from.Spec.Template.Spec.Containers[0].Env) {
		log.V(1).Info("reconciling Job due to container[0] env change")
		log.V(2).Info("difference in Job container[0] env", "wanted", from.Spec.Template.Spec.Containers[0].Env, "existing", to.Spec.Template.Spec.Containers[0].Env)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].Env = from.Spec.Template.Spec.Containers[0].Env

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].EnvFrom, from.Spec.Template.Spec.Containers[0].EnvFrom) {
		log.V(1).Info("reconciling Job due to container[0] EnvFrom change")
		log.V(2).Info("difference in Job container[0] EnvFrom", "wanted", from.Spec.Template.Spec.Containers[0].EnvFrom, "existing", to.Spec.Template.Spec.Containers[0].EnvFrom)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].EnvFrom = from.Spec.Template.Spec.Containers[0].EnvFrom

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].Resources, from.Spec.Template.Spec.Containers[0].Resources) {
		log.V(1).Info("reconciling Job due to container[0] resource change")
		log.V(2).Info("difference in Job container[0] resources", "wanted", from.Spec.Template.Spec.Containers[0].Resources, "existing", to.Spec.Template.Spec.Containers[0].Resources)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].Resources = from.Spec.Template.Spec.Containers[0].Resources

	if !reflect.DeepEqual(to.Spec.Template.Spec.Containers[0].VolumeMounts, from.Spec.Template.Spec.Containers[0].VolumeMounts) {
		log.V(1).Info("reconciling Job due to container[0] VolumeMounts change")
		log.V(2).Info("difference in Job container[0] VolumeMounts", "wanted", from.Spec.Template.Spec.Containers[0].VolumeMounts, "existing", to.Spec.Template.Spec.Containers[0].VolumeMounts)
		requireUpdate = true
	}
	to.Spec.Template.Spec.Containers[0].VolumeMounts = from.Spec.Template.Spec.Containers[0].VolumeMounts

	return requireUpdate
}
