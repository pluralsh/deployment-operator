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
	"fmt"
	"reflect"

	"github.com/google/uuid"

	"encoding/json"

	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	console "github.com/pluralsh/console-client-go"
	pipelinesv1alpha1 "github.com/pluralsh/deployment-operator/apis/pipelines/v1alpha1"

	//job "k8s.io/api/batch/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineGateReconciler reconciles a PipelineGate object
type PipelineGateReconciler struct {
	client.Client
	ConsoleClient *consoleclient.Client
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
	// ASSUMPTION: GATE AT CONSOLE CANNOT BE SET TO CLOSED OR OPEN BY USER
	_ = log.FromContext(ctx)

	log := r.Log.WithValues("PipelineGate", req.NamespacedName)

	gate := &pipelinesv1alpha1.PipelineGate{}

	// get pipelinegate
	if err := r.Get(ctx, req.NamespacedName, gate); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Unable to fetch PipelineGate - skipping", "Namespace", gate.Namespace, "Name", gate.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch PipelineGate")
		return ctrl.Result{}, err
	}

	// INITIAL STATE /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// on initial reconcile we set the state to pending
	if gate.Status.State == nil {
		// update CR state
		gate.Status.State = &gate.Spec.SyncedState
		if err := r.Status().Update(ctx, gate); err != nil {
			log.Error(err, "Failed to update PipelineGate status at CR")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// PENDING ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	if gate.Status.State != nil && *gate.Status.State == pipelinesv1alpha1.GateState(console.GateStatePending) {
		if gate.Status.JobRef == nil { // if there is no jobRef, then we need to create a job, this means this is either a new gate or a rerun
			jobName := fmt.Sprintf("%s-%s", gate.Name, uuid.New().String())
			jobRef := console.NamespacedName{Name: jobName, Namespace: gate.Namespace}
			job := r.generateJob(ctx, log, *gate.Spec.GateSpec.JobSpec, jobRef)
			if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
				log.Error(err, "Error setting ControllerReference for Job")
				return ctrl.Result{}, err
			}
			// reconcile job
			_, err := Job(ctx, r.Client, job, log)
			if err != nil {
				log.Error(err, "Error reconciling Job", "Job", job.Name, "Namespace", job.Namespace)
				return ctrl.Result{}, err
			}
			gateState := pipelinesv1alpha1.GateState(console.GateStatePending)
			gate.Status.State = &gateState
			gate.Status.JobRef = &jobRef
			if err := r.Status().Update(ctx, gate); err != nil {
				log.Error(err, "Failed to update PipelineGate status at CR")
				return ctrl.Result{}, err
			}
			log.Info("Updated gate state after reconciling job", "Name", gate.Name, "ID", gate.Spec.ID, "State", gateState)
			return ctrl.Result{}, nil
		} else { // if there is a jobRef, then we need to check the job status
			job := r.generateJob(ctx, log, *gate.Spec.GateSpec.JobSpec, *gate.Status.JobRef)
			if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
				log.Error(err, "Error setting ControllerReference for Job")
				return ctrl.Result{}, err
			}
			// reconcile job, creates a new one or gets the old one
			reconciledJob, err := Job(ctx, r.Client, job, log)
			if err != nil {
				log.Error(err, "Error reconciling Job", "Job", job.Name, "Namespace", job.Namespace)
				return ctrl.Result{}, err
			}

			var gateState pipelinesv1alpha1.GateState
			if failed, condition := hasFailed(reconciledJob); failed {
				// if the job is failed, then we need to update the gate state to closed, unless it's a rerun
				log.Info("Job failed", "Name", job.Name, "Namespace", job.Namespace, "Condition", condition)
				gateState = pipelinesv1alpha1.GateState(console.GateStateClosed)
			} else if succeeded, condition := hasSucceeded(reconciledJob); succeeded {
				// if the job is complete, then we need to update the gate state to open, unless it's a rerun
				log.Info("Job succeeded", "Name", job.Name, "Namespace", job.Namespace, "Condition", condition)
				gateState = pipelinesv1alpha1.GateState(console.GateStateOpen)
			} else {
				// if the job is still running, then we need to do nothing
				log.Info("Job is still running", "Name", job.Name, "Namespace", job.Namespace, "Condition", condition)
				gateState = pipelinesv1alpha1.GateState(console.GateStatePending)
			}
			gate.Status.State = &gateState
			if err := r.Status().Update(ctx, gate); err != nil {
				log.Error(err, "Failed to update PipelineGate status at CR")
				return ctrl.Result{}, err
			}
			log.Info("Updated gate state after reconciling job", "Name", gate.Name, "ID", gate.Spec.ID, "State", gateState)
			return ctrl.Result{}, nil
		}
	}

	// PUSH SYNC ////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// gate is open or closed as per last reconciliation
	if gate.Status.State != nil &&
		(gate.Status.LastReported == nil || *gate.Status.LastReported == pipelinesv1alpha1.GateState(console.GateStatePending)) &&
		(*gate.Status.State == pipelinesv1alpha1.GateState(console.GateStateOpen) || *gate.Status.State == pipelinesv1alpha1.GateState(console.GateStateClosed)) {

		// def state var for update
		var gateState console.GateState
		if *gate.Status.State == pipelinesv1alpha1.GateState(console.GateStateOpen) {
			gateState = console.GateStateOpen
		} else {
			gateState = console.GateStateClosed
		}
		log.Info("PipelineGate State changed", "Namespace", gate.Namespace, "Name", gate.Name, "State", gate.Status.State)

		updateAttributes := console.GateUpdateAttributes{State: &gateState, Status: &console.GateStatusAttributes{JobRef: gate.Status.JobRef}}
		// DEBUG
		gateJSON, err := json.MarshalIndent(updateAttributes, "", "  ")
		if err != nil {
			log.Error(err, "failed to marshalindent updateAttributes")
		}
		fmt.Printf("updateAttributes json from API: \n %s\n", string(gateJSON))
		// DEBUG
		// try to update gate at console
		if err := r.ConsoleClient.UpdateGate(gate.Spec.ID, updateAttributes); err != nil {
			log.Error(err, "Failed to update PipelineGate status to console")
			return ctrl.Result{}, err
		}
		log.Info("Updated gate state at console", "Name", gate.Name, "ID", gate.Spec.ID, "State", gateState)

		// update CR state
		lastReportedAt := metav1.Now()
		gate.Status.LastReportedAt = &lastReportedAt
		lastReported := pipelinesv1alpha1.GateState(gateState)
		gate.Status.LastReported = &lastReported
		if err := r.Status().Update(ctx, gate); err != nil {
			log.Error(err, "Failed to update PipelineGate status at CR")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// SCHEDULE RERUN /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// gate is closed or open as per last reconciliation, but got updated from console to pending, for a rerun of the pipeline for example
	if gate.Status.State != nil && // before we compare these, we need to make sure they are not nil
		gate.Status.LastReported != nil &&
		gate.Status.LastReportedAt != nil &&
		gate.Spec.LastSyncedAt != nil &&
		// how to determine if there was a re-run
		// 1) the state is open or closed
		// 2) the last reported state is open or closed, by our knowledge this was the last state at the console
		// 3) the last sync from console (agent pull) happened AFTER we last reported the state
		// 4) the state at the console is pending, as reflecte by synced state
		((*gate.Status.State == pipelinesv1alpha1.GateState(console.GateStateOpen) && *gate.Status.LastReported == pipelinesv1alpha1.GateState(console.GateStateOpen)) ||
			(*gate.Status.State == pipelinesv1alpha1.GateState(console.GateStateClosed) && *gate.Status.LastReported == pipelinesv1alpha1.GateState(console.GateStateClosed))) &&
		(*gate.Status.LastReportedAt).Before(gate.Spec.LastSyncedAt) &&
		gate.Spec.SyncedState == pipelinesv1alpha1.GateState(console.GateStatePending) {
		// update CR state
		gate.Status.State = &gate.Spec.SyncedState // will be pending
		// set jobRef to nil, this is important to make sure we create a new job
		gate.Status.JobRef = nil
		if err := r.Status().Update(ctx, gate); err != nil {
			log.Error(err, "Failed to update PipelineGate status at CR")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func hasFailed(job *batchv1.Job) (bool, *batchv1.JobCondition) {
	conditions := job.Status.Conditions
	// check if the conditions slice contains a failed condition
	// failed means the backoff limit was reached and it is not being retried anymore, so it's failed for real!
	for _, condition := range conditions {
		if condition.Type == batchv1.JobFailed {
			return true, &condition
		}
	}
	return false, nil
}

func hasSucceeded(job *batchv1.Job) (bool, *batchv1.JobCondition) {
	conditions := job.Status.Conditions
	// check if the conditions slice contains a failed condition
	for _, condition := range conditions {
		if condition.Type == batchv1.JobComplete {
			return true, &condition
		}
	}
	return false, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelinesv1alpha1.PipelineGate{}).
		Owns(&batchv1.Job{}).
		Complete(r)

}

// Job reconciles a k8s job object.
func Job(ctx context.Context, r client.Client, job *batchv1.Job, log logr.Logger) (*batchv1.Job, error) {
	foundJob := &batchv1.Job{}
	justCreated := false
	if err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if apierrs.IsNotFound(err) {
			log.Info("Creating Job", "namespace", job.Namespace, "name", job.Name)
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Unable to create Job")
				return nil, err
			}
			justCreated = true
		} else {
			log.Error(err, "Error getting Job")
			return nil, err
		}
	}
	if !justCreated && CopyJobFields(job, foundJob, log) {
		log.Info("Updating Job", "namespace", job.Namespace, "name", job.Name)
		if err := r.Update(ctx, foundJob); err != nil {
			log.Error(err, "Unable to update Job")
			return foundJob, err
		}
	}
	if justCreated {
		return job, nil
	}
	return foundJob, nil
}

func (r *PipelineGateReconciler) generateJob(ctx context.Context, log logr.Logger, jobSpec batchv1.JobSpec, jobRef console.NamespacedName) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRef.Name,
			Namespace: jobRef.Namespace,
		},
		Spec: jobSpec,
	}
}

func CopyJobFields(from, to *batchv1.Job, log logr.Logger) bool {
	requireUpdate := false
	//for k, v := range to.Labels {
	//	if from.Labels[k] != v {
	//		log.V(1).Info("reconciling Job due to label change")
	//		log.V(2).Info("difference in Job labels", "wanted", from.Labels, "existing", to.Labels)
	//		requireUpdate = true
	//	}
	//}
	//if len(to.Labels) == 0 && len(from.Labels) != 0 {
	//	log.V(1).Info("reconciling Job due to label change")
	//	log.V(2).Info("difference in Job labels", "wanted", from.Labels, "existing", to.Labels)
	//	requireUpdate = true
	//}
	//to.Labels = from.Labels

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

	//for k, v := range to.Spec.Template.Labels {
	//	if from.Spec.Template.Labels[k] != v {
	//		log.V(1).Info("reconciling Job due to template label change")
	//		log.V(2).Info("difference in Job template labels", "wanted", from.Spec.Template.Labels, "existing", to.Spec.Template.Labels)
	//		requireUpdate = true
	//	}
	//}
	//if len(to.Spec.Template.Labels) == 0 && len(from.Spec.Template.Labels) != 0 {
	//	log.V(1).Info("reconciling Job due to template label change")
	//	log.V(2).Info("difference in Job template labels", "wanted", from.Spec.Template.Labels, "existing", to.Spec.Template.Labels)
	//	requireUpdate = true
	//}
	//to.Spec.Template.Labels = from.Spec.Template.Labels

	//for k, v := range to.Spec.Template.Annotations {
	//	if from.Spec.Template.Annotations[k] != v {
	//		log.V(1).Info("reconciling Job due to template annotation change")
	//		log.V(2).Info("difference in Job template annotations", "wanted", from.Spec.Template.Annotations, "existing", to.Spec.Template.Annotations)
	//		requireUpdate = true
	//	}
	//}
	//if len(to.Spec.Template.Annotations) == 0 && len(from.Spec.Template.Annotations) != 0 {
	//	log.V(1).Info("reconciling Job due to template annotation change")
	//	log.V(2).Info("difference in Job template annotations", "wanted", from.Spec.Template.Annotations, "existing", to.Spec.Template.Annotations)
	//	requireUpdate = true
	//}
	//to.Spec.Template.Annotations = from.Spec.Template.Annotations

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
