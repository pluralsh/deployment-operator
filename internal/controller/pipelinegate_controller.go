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

package controller

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"

	"github.com/go-logr/logr"
	console "github.com/pluralsh/console-client-go"
	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PipelineGateFinalizer = "deployments.plural.sh/pipelinegate-protection"
)

// PipelineGateReconciler reconciles a PipelineGate object
type PipelineGateReconciler struct {
	client.Client
	ConsoleClient consoleclient.Client
	GateCache     *consoleclient.Cache[console.PipelineGateFragment]
	Scheme        *runtime.Scheme
	Log           logr.Logger
}

//+kubebuilder:rbac:groups=deployments.plural.sh,resources=pipelinegates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=pipelinegates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=pipelinegates/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;deletecollection

/*
cases:
- k8s: pending with no job ref
	- cache: console pending with no job ref
		-> hash is the same
			-> reconcile:
				k8s: create job, set job ref (this has to be atomic, can't set job ref on job event, because there might be multiple jobs running for the same gate, and we can't easily know which is the latest)
				console: do nothing
- k8s: pending with job ref
	- k8s: job is running
		- cache: console pending with job ref and job ref is the same
			-> hash is the same
				-> no reconcile
		- cache: console pending with no job ref
			-> hash is different
			-> reconcile:
				k8s: do nothing
				console: update job ref at console, should show up in hash
		- cache: console pending with job ref -> hash is different (probably a rerun, it was reset to pending at the console and a new job was already created at k8s, but the console doesn't know about it yet)
			-> reconcile:
				k8s: do nothing
				console: update job ref at console, should show up in hash
	- k8s: job has succeeded
		- cache: console pending with job ref -> hash is different
			-> reoconcile
		- cache: console open with job ref
			-> reconcile:

	- job has failed
- k8s open with job ref
- k8s closed with job ref
- console pending with no job ref
	- k8s pending with job ref (job has been created, but not reported yet)
	- k8s open with job ref (if job is complete, but has not been reported yet)
- console pending with job ref
- console open with job ref
- console closed with job ref

linear issue:

1) have the poller/reconciler against the console api only create the PipelineGate cr and should share it's gate cache w/ the CRD reconciler (this can help reduce concerns over read pressure).
Currently it's also doing some stuff to control writes to the console that is likely too complex to pair w/ the main reconciliation loop.

2) The CRD reconciler loop should fetch the current gate by id, see its state, and if it requires a new job to be created, create it, otherwise update it in case a new spec has been issued.
 	If a created job has a new status, it should report it.  This should be pretty easy to determine by comparing upstream versus current gate statuses.  Generally the transitions should be

// i think this is not possible:
3) if upstream gate is pending and no job is running, start one if upstream gate is closed/open and job is running, cancel it if possible, otherwise nothing needs to be done
how to determine "no job is running":
- has no jobref in status
- has a jobref in status, but job is not running (failed or succeeded, or crashed)
scenario:
- k8s: pending with jobref of failed job (reconcile trigger was change of job status to completed)
- console as per cache: pending with jobref of now failed job (but no change in sha of status, so no console update)
- if I know check the job status, it's failed, so I need to set the status to closed
- on the next reconcile, I will see that the status is closed, and the sha is different, so I will update the console
- I have now chance to recreate the job, when the console gate is pending, because it might be just

basically the console might just be waiting for a the crd reconciler to report back the failure of the job or it might not have gotten the jobref yet

how do I know if the gate at the console was reset to pending?
- just being in the pending state is not enough!
- I cannot rely on the status of the referenced job in the cluster because it might not have been reported yet!
- I

We should use the same strategy as with the console controller to determine whether status should be reported back to the api,
eg keep a sha of the last applied status and if the current one we built diverged, apply it.
This will ensure we only issue write requests once whenever necessary, and we can attempt an update to the console on any reconcile loop safely w/ it.

*/

func (r *PipelineGateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx).WithValues("PipelineGate", req.NamespacedName)

	crGate := &v1alpha1.PipelineGate{}
	if err := r.Get(ctx, req.NamespacedName, crGate); err != nil {
		if apierrs.IsNotFound(err) {
			log.V(1).Info("PipelineGate CR not found - skipping.", "Namespace", crGate.Namespace, "Name", crGate.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "Unable to fetch PipelineGate.")
		return ctrl.Result{}, err
	}

	cachedGate, err := r.GateCache.Get(crGate.Spec.ID)
	if err != nil {
		log.Error(err, "Unable to fetch PipelineGate from cache, this gate probably doesn't exist in the console.")
		return ctrl.Result{}, err
	}

	_, err = r.sync(ctx, crGate, *cachedGate)

	scope, _ := NewPipelineGateScope(ctx, r.Client, crGate)
	defer func() {
		// culculate sha and update status
		attrs, err := r.gateUpdateAttributesFromPipelineGateCR(ctx, crGate)
		cachedSha, err := utils.HashObject(gateUpdateAttributesFromPipelineGateFragment(cachedGate))
		crGate.Status.SHA = &sha
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// INITIAL STATE
	//if !crGate.Status.IsInitialized() {
	//	return r.initializeGateState(ctx, crGate, log)
	//}

	// PENDING
	if crGate.Status.IsPending() {
		result, err := r.reconcilePendingGate(ctx, crGate, *cachedGate)
		crGate.Status.SHA = &sha
		return result, err
	}

	// PUSH SYNC
	if crGate.Status.IsOpen() || crGate.Status.IsClosed() {
		result, err := r.reconcilePendingGate(ctx, crGate, *cachedGate)
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *PipelineGateReconciler) sync(ctx context.Context, crGate *v1alpha1.PipelineGate, cachedGate console.PipelineGateFragment) (bool, error) {
	logger := log.FromContext(ctx)
	cachedSha, err := utils.HashObject(gateUpdateAttributesFromPipelineGateFragment(cachedGate))
	if err != nil {
		return false, err
	}

	if crGate.Status.IsSHAEqual(cachedSha) {
		logger.V(2).Info(fmt.Sprintf("No changes detected for gate %s ", crGate.Name))
		return false, nil
	}

	// something changed!

	updateAttrs, err := r.gateUpdateAttributesFromPipelineGateCR(ctx, crGate)

	if err := r.ConsoleClient.UpdateGate(crGate.Spec.ID, *updateAttrs); err != nil {
		logger.Error(err, "Failed to update PipelineGate status to console")
		return true, err
	}
	if _, err = r.GateCache.Set(crGate.Spec.ID); err != nil {
		logger.Error(err, "Failed to update GateCache immediately")
	}
	return true, nil
}

func gateUpdateAttributesFromPipelineGateFragment(fragment console.PipelineGateFragment) console.GateUpdateAttributes {
	var jobRef *console.NamespacedName
	if fragment.Job != nil && fragment.Job.Metadata.Namespace != nil {
		jobRef = &console.NamespacedName{
			Name:      fragment.Job.Metadata.Name,
			Namespace: *fragment.Job.Metadata.Namespace,
		}
	} else if fragment.Job != nil {
		jobRef = &console.NamespacedName{
			Name:      fragment.Job.Metadata.Name,
			Namespace: "",
		}
	} else {
		jobRef = &console.NamespacedName{}
	}

	return console.GateUpdateAttributes{
		State: &fragment.State,
		Status: &console.GateStatusAttributes{
			JobRef: jobRef,
		},
	}
}

func (r *PipelineGateReconciler) gateUpdateAttributesFromPipelineGateCR(ctx context.Context, pg *v1alpha1.PipelineGate) (*console.GateUpdateAttributes, error) {
	state, err := pg.Status.GetConsoleGateState()
	if err != nil {
		return nil, err
	}
	updateAttributes := console.GateUpdateAttributes{State: state, Status: &console.GateStatusAttributes{JobRef: pg.Status.JobRef}}
	return &updateAttributes, nil
}

func (r *PipelineGateReconciler) initializeGateState(ctx context.Context, gate *v1alpha1.PipelineGate, log logr.Logger) (ctrl.Result, error) {
	if gate.Status.State == nil {
		// update CR state
		gate.Status.State = &gate.Status.SyncedState
		if err := r.Status().Update(ctx, gate); err != nil {
			log.Error(err, "Failed to update PipelineGate status at CR")
			return ctrl.Result{}, err
		}
		log.V(1).Info("Updated state of CR on first reconcile.", "Namespace", gate.Namespace, "Name", gate.Name, "ID", gate.Spec.ID, "SyncedState", gate.Status.SyncedState, "LastSyncedAt", gate.Status.LastSyncedAt, "State", gate.Status.State)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

func (r *PipelineGateReconciler) reconcilePendingGate(ctx context.Context, gate *v1alpha1.PipelineGate, cachedGate console.PipelineGateFragment) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Reconciling PENDING gate.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State)
	//consoleGate := r.GateCache.Get(gate.Spec.ID)
	if !gate.Status.HasJobRef() {
		log.V(2).Info("Gate doesn't have a JobRef, this is a new gate or a re-run.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State)

		jobName := fmt.Sprintf("%s-%s", gate.Name, uuid.New().String())
		jobRef := console.NamespacedName{Name: jobName, Namespace: gate.Namespace}
		job := r.generateJob(ctx, log, *gate.Spec.GateSpec.JobSpec, jobRef)
		if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
			log.Error(err, "Error setting ControllerReference for Job.")
			return ctrl.Result{}, err
		}

		// reconcile job

		log.V(2).Info("Creating new job for gate.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "jobRef", jobRef)
		_, err := Job(ctx, r.Client, job, log)
		if err != nil {
			log.Error(err, "Error reconciling Job.", "JobName", job.Name, "Namespace", job.Namespace)
			return ctrl.Result{}, err
		}

		gateState := v1alpha1.GateState(console.GateStatePending)
		gate.Status.State = &gateState
		gate.Status.JobRef = &jobRef
		//if err := r.Status().Update(ctx, gate); err != nil {
		//	log.Error(err, "Failed to update PipelineGate status at CR.")
		//	return ctrl.Result{}, err
		//}

		// try to update gate at console
		//attributeState := console.GateStatePending
		//updateAttributes := console.GateUpdateAttributes{State: &attributeState, Status: &console.GateStatusAttributes{JobRef: gate.Status.JobRef}}
		//if err := r.ConsoleClient.UpdateGate(gate.Spec.ID, updateAttributes); err != nil {
		//	log.Error(err, "Failed to update PipelineGate status to console")
		//}

		//log.V(1).Info("Created new job for gate and updated status.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "jobRef", *gate.Status.JobRef)
		return ctrl.Result{}, nil
	} else {
		log.V(2).Info("Gate has a JobRef, checking Job status.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "jobRef", *gate.Status.JobRef)
		job := r.generateJob(ctx, log, *gate.Spec.GateSpec.JobSpec, *gate.Status.JobRef)
		if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
			log.Error(err, "Error setting ControllerReference for Job.")
			return ctrl.Result{}, err
		}
		// reconcile job, creates a new one or gets the old one
		reconciledJob, err := Job(ctx, r.Client, job, log)
		if err != nil {
			log.Error(err, "Error reconciling Job.", "JobName", job.Name, "JobNamespace", job.Namespace)
			return ctrl.Result{}, err
		}

		var gateState v1alpha1.GateState
		if failed := hasFailed(reconciledJob); failed {
			// if the job is failed, then we need to update the gate state to closed, unless it's a rerun
			log.V(2).Info("Job failed.", "JobName", job.Name, "JobNamespace", job.Namespace)
			gateState = v1alpha1.GateState(console.GateStateClosed)
		} else if succeeded := hasSucceeded(reconciledJob); succeeded {
			// if the job is complete, then we need to update the gate state to open, unless it's a rerun
			log.V(1).Info("Job succeeded.", "JobName", job.Name, "JobNamespace", job.Namespace)
			gateState = v1alpha1.GateState(console.GateStateOpen)
		} else {
			// if the job is still running, then we need to do nothing
			log.V(1).Info("Job is still running.", "JobName", job.Name, "JobNamespace", job.Namespace)
			gateState = v1alpha1.GateState(console.GateStatePending)
		}
		gate.Status.State = &gateState
		//if err := r.Status().Update(ctx, gate); err != nil {
		//	log.Error(err, "Failed to update PipelineGate status at CR")
		//	return ctrl.Result{}, err
		//}
		//log.V(1).Info("Updated gate status after reconciling job.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "jobRef", *gate.Status.JobRef)
		return ctrl.Result{}, nil
	}
}

//func (r *PipelineGateReconciler) syncGateStatus(ctx context.Context, gate *v1alpha1.PipelineGate, log logr.Logger) (ctrl.Result, error) {
//
//	log.V(1).Info(fmt.Sprintf("Reconciling %s gate.", string(*gate.Status.State)), "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "LastSyncedAt", gate.Status.LastSyncedAt, "jobRef", *gate.Status.JobRef)
//
//	var gateState console.GateState
//	if *gate.Status.State == v1alpha1.GateState(console.GateStateOpen) {
//		gateState = console.GateStateOpen
//	} else {
//		gateState = console.GateStateClosed
//	}
//
//	updateAttributes := console.GateUpdateAttributes{State: &gateState, Status: &console.GateStatusAttributes{JobRef: gate.Status.JobRef}}
//
//	if err := r.ConsoleClient.UpdateGate(gate.Spec.ID, updateAttributes); err != nil {
//		log.Error(err, "Failed to update PipelineGate status to console")
//		return ctrl.Result{}, err
//	}
//
//	lastReportedAt := metav1.Now()
//	lastReported := v1alpha1.GateState(gateState)
//	log.V(2).Info(fmt.Sprintf("Updated gate state to %s console.", string(*gate.Status.State)), "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "lastReported", lastReported, "LastReportedAt", lastReportedAt, "LastSyncedAt", gate.Status.LastSyncedAt, "jobRef", *gate.Status.JobRef)
//
//	gate.Status.LastReportedAt = &lastReportedAt
//	gate.Status.LastReported = &lastReported
//	if err := r.Status().Update(ctx, gate); err != nil {
//		log.Error(err, "Failed to update PipelineGate status at CR")
//		return ctrl.Result{}, err
//	}
//
//	log.V(2).Info(fmt.Sprintf("Updated gate state lastReported to %s.", string(lastReported)), "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State, "LastReportedAt", *gate.Status.LastReportedAt, "LastSyncedAt", gate.Status.LastSyncedAt, "jobRef", *gate.Status.JobRef)
//
//	return ctrl.Result{}, nil
//}

// IsStatusConditionTrue returns true when the conditionType is present and set to `metav1.ConditionTrue`
func IsJobStatusConditionTrue(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	return IsJobStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
}

// IsStatusConditionFalse returns true when the conditionType is present and set to `metav1.ConditionFalse`
func IsJobStatusConditionFalse(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	return IsJobStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionFalse)
}

// IsStatusConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsJobStatusConditionPresentAndEqual(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType, status corev1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

func hasFailed(job *batchv1.Job) bool {
	return IsJobStatusConditionTrue(job.Status.Conditions, batchv1.JobFailed)
}

func hasSucceeded(job *batchv1.Job) bool {
	return IsJobStatusConditionTrue(job.Status.Conditions, batchv1.JobComplete)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PipelineGateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PipelineGate{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// Job reconciles a k8s job object.
func Job(ctx context.Context, r client.Client, job *batchv1.Job, log logr.Logger) (*batchv1.Job, error) {
	foundJob := &batchv1.Job{}
	justCreated := false
	if err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if apierrs.IsNotFound(err) {
			log.V(2).Info("Creating Job.", "Namespace", job.Namespace, "Name", job.Name)
			if err := r.Create(ctx, job); err != nil {
				log.Error(err, "Unable to create Job.")
				return nil, err
			}
			justCreated = true
		} else {
			log.Error(err, "Error getting Job.")
			return nil, err
		}
	}
	if !justCreated && CopyJobFields(job, foundJob, log) {
		log.V(2).Info("Updating Job.", "Namespace", job.Namespace, "Name", job.Name)
		if err := r.Update(ctx, foundJob); err != nil {
			if apierrs.IsConflict(err) {
				return foundJob, nil
			}

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

//func (r *PipelineGateReconciler) addOrRemoveFinalizer(pg *v1alpha1.PipelineGate) *ctrl.Result {
//	/// If object is not being deleted and if it does not have our finalizer,
//	// then lets add the finalizer. This is equivalent to registering our finalizer.
//	if pg.ObjectMeta.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(pg, PipelineGateFinalizer) {
//		controllerutil.AddFinalizer(pg, PipelineGateFinalizer)
//	}
//
//	// If object is being deleted cleanup and remove the finalizer.
//	if !pg.ObjectMeta.DeletionTimestamp.IsZero() {
//		// Remove PipelineGate from Console API if it exists.
//		if r.ConsoleClient.GateExists(pg.Spec.ID) {
//			if _, err := r.ConsoleClient.DeletePipeline(*pipeline.Status.ID); err != nil {
//				// If it fails to delete the external dependency here, return with error
//				// so that it can be retried.
//				utils.MarkCondition(pipeline.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
//				return &ctrl.Result{}
//			}
//
//			// If deletion process started requeue so that we can make sure provider
//			// has been deleted from Console API before removing the finalizer.
//			return &requeue
//		}
//
//		// If our finalizer is present, remove it.
//		controllerutil.RemoveFinalizer(pipeline, PipelineFinalizer)
//
//		// Stop reconciliation as the item is being deleted.
//		return &ctrl.Result{}
//	}
//
//	return nil
//}
