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
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/samber/lo"
	"time"

	"github.com/go-logr/logr"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	if !crGate.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	cachedGate, err := r.GateCache.Get(crGate.Spec.ID)
	if err != nil {
		log.Info("Unable to fetch PipelineGate from cache, this gate probably doesn't exist in the console.")
		if err := r.Delete(ctx, crGate); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	scope, err := NewPipelineGateScope(ctx, r.Client, crGate)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
			return
		}
	}()

	// INITIAL STATE
	if !crGate.Status.IsInitialized() {
		crGate.Status.SetState(console.GateStatePending)
		log.V(1).Info("Updated state of CR on first reconcile.", "Namespace", crGate.Namespace, "Name", crGate.Name, "ID", crGate.Spec.ID)
		return ctrl.Result{}, nil
	}

	// PENDING
	if crGate.Status.IsPending() {
		return r.reconcilePendingGate(ctx, crGate, cachedGate)
	}

	// RERUN
	if (crGate.Status.IsOpen() || crGate.Status.IsClosed()) && crGate.Status.HasJobRef() && consoleclient.IsPending(cachedGate) {
		crGate.Status.SetState(console.GateStatePending)
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cachedGate.Status.JobRef.Name,
				Namespace: cachedGate.Status.JobRef.Namespace,
			},
		}
		if err := killJob(ctx, r.Client, job); err != nil {
			return ctrl.Result{}, err
		}
		crGate.Status.JobRef = nil
		return ctrl.Result{}, nil
	}

	return requeue, nil
}

func killJob(ctx context.Context, c client.Client, job *batchv1.Job) error {
	log := log.FromContext(ctx)
	deletePolicy := metav1.DeletePropagationBackground // kill the job and its pods asap
	if err := c.Delete(ctx, job, &client.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
		return nil
	}
	log.V(2).Info("Job killed successfully.", "JobName", job.Name, "Namespace", job.Namespace)
	return nil
}

func (r *PipelineGateReconciler) reconcilePendingGate(ctx context.Context, gate *v1alpha1.PipelineGate, cachedGate *console.PipelineGateFragment) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("Reconciling PENDING gate.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State)
	jobSpec := consoleclient.JobSpecFromJobSpecFragment(cachedGate.Name, cachedGate.Spec.Job)
	jobRef := gate.CreateNewJobRef()
	job := generateJob(*jobSpec, jobRef)
	sha, err := utils.HashObject(job.Spec.Template.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	gate.Spec.GateSpec.JobSpec = lo.ToPtr(job.Spec)
	reconciledJob, err := Job(ctx, r.Client, job, log)
	if err != nil {
		log.Error(err, "Error reconciling Job.", "JobName", job.Name, "JobNamespace", job.Namespace)
		return ctrl.Result{}, err
	}

	if !gate.Status.HasJobRef() {
		log.V(2).Info("Gate doesn't have a JobRef, this is a new gate or a re-run.", "Name", gate.Name, "ID", gate.Spec.ID, "State", *gate.Status.State)
		if err := ctrl.SetControllerReference(gate, job, r.Scheme); err != nil {
			log.Error(err, "Error setting ControllerReference for Job.")
			return ctrl.Result{}, err
		}

		gate.Status.SetState(console.GateStatePending)
		gate.Status.JobRef = lo.ToPtr(jobRef)
		gate.Status.SetSHA(sha)
		return ctrl.Result{}, nil
	}

	// update when job is in progress
	if !gate.Status.IsSHAEqual(sha) {
		//reconciledJob.Spec.Template.Spec.Containers = job.Spec.Template.Spec.Containers
		if err := r.Client.Update(ctx, reconciledJob); err != nil {
			if apierrs.IsConflict(err) {
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
			return ctrl.Result{}, err
		}
		gate.Status.SetSHA(sha)
	}

	// ABORT:
	if consoleclient.IsClosed(cachedGate) {
		// I don't think a guarantee for aborting a job is possible, unless we change the console api to allow for it
		// try to kill the job
		if err := killJob(ctx, r.Client, job); err != nil {
			return ctrl.Result{}, err
		}
		// even if the killing of the job fails, we better update the gate status to closed asap, so we don't report a gate CR transition from pending to closed
		gate.Status.SetState(console.GateStateClosed)
		gate.Status.JobRef = nil
		log.V(1).Info("Job aborted.", "JobName", job.Name, "JobNamespace", job.Namespace)
		return requeue, nil
	}

	// check job status
	if hasFailed(reconciledJob) {
		// if the job is failed, then we need to update the gate state to closed, unless it's a rerun
		log.V(2).Info("Job failed.", "JobName", job.Name, "JobNamespace", job.Namespace)
		gate.Status.SetState(console.GateStateClosed)
		if err := r.updateConsoleGate(gate); err != nil {
			return ctrl.Result{}, err
		}
	}
	if hasSucceeded(reconciledJob) {
		// if the job is complete, then we need to update the gate state to open, unless it's a rerun
		log.V(1).Info("Job succeeded.", "JobName", job.Name, "JobNamespace", job.Namespace)
		gate.Status.SetState(console.GateStateOpen)
		if err := r.updateConsoleGate(gate); err != nil {
			return ctrl.Result{}, err
		}
	}

	return requeue, nil
}

// IsStatusConditionTrue returns true when the conditionType is present and set to `metav1.ConditionTrue`
func IsJobStatusConditionTrue(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) bool {
	return IsJobStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
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
	if err := r.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		log.V(2).Info("Creating Job.", "Namespace", job.Namespace, "Name", job.Name)
		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Unable to create Job.")
			return nil, err
		}
		return job, nil
	}
	return foundJob, nil

}

func generateJob(jobSpec batchv1.JobSpec, jobRef console.NamespacedName) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRef.Name,
			Namespace: jobRef.Namespace,
		},
		Spec: jobSpec,
	}
}

func (r *PipelineGateReconciler) updateConsoleGate(gate *v1alpha1.PipelineGate) error {
	updateAttrs, err := gate.Status.GateUpdateAttributes()
	if err != nil {
		return err
	}
	if err := r.ConsoleClient.UpdateGate(gate.Spec.ID, *updateAttrs); err != nil {
		return err
	}
	if _, err = r.GateCache.Set(gate.Spec.ID); err != nil {
		return err
	}
	return nil
}
