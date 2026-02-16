package controller

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/samber/lo"
	batchv1 "k8s.io/api/batch/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const SentinelRunJobFinalizerName = "sentinelrunjob.deployments.plural.sh/finalizer"

type SentinelRunJobReconciler struct {
	client.Client
	consoleClient consoleclient.Client
	Scheme        *runtime.Scheme
	consoleURL    string
	deployToken   string
}

func (r *SentinelRunJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, retErr error) {
	fromContext := log.FromContext(ctx)
	fromContext.Info("Reconciling SentinelRunJob", "name", req.Name, "namespace", req.Namespace)

	srj := &v1alpha1.SentinelRunJob{}
	if err := r.Get(ctx, req.NamespacedName, srj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scope, err := NewDefaultScope(ctx, r.Client, srj)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	utils.MarkCondition(srj.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(srj.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReason, "")

	result := r.addOrRemoveFinalizer(ctx, srj)
	if result != nil {
		return *result, nil
	}

	run, err := r.consoleClient.GetSentinelRunJob(srj.Spec.RunID)
	if err != nil {
		return ctrl.Result{}, err
	}

	secret, err := r.reconcileRunSecret(ctx, req.Name, req.Namespace, srj.Spec.RunID, string(run.Format))
	if err != nil {
		return ctrl.Result{}, err
	}

	job, err := r.reconcileRunJob(ctx, srj, run)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := utils.TryAddOwnerRef(ctx, r, job, secret, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	if err := utils.TryAddControllerRef(ctx, r, srj, job, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	unstructuredJob, err := common.ToUnstructured(job)
	if err != nil {
		return ctrl.Result{}, err
	}
	health, err := common.GetResourceHealth(unstructuredJob)
	if err != nil {
		return ctrl.Result{}, err
	}

	var status *console.SentinelRunJobStatus
	if health != nil {
		srj.Status.JobStatus = string(health.Status)
		if health.Status == common.HealthStatusDegraded {
			status = lo.ToPtr(console.SentinelRunJobStatusFailed)
		}
	}

	if err := r.consoleClient.UpdateSentinelRunJobStatus(srj.Spec.RunID, &console.SentinelRunJobUpdateAttributes{
		Status: status,
		Reference: &console.NamespacedName{
			Name:      job.Name,
			Namespace: job.Namespace,
		},
	}); err != nil {
		return ctrl.Result{}, err
	}

	srj.Status.ID = lo.ToPtr(run.ID)
	utils.MarkCondition(srj.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(srj.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionTrue, v1alpha1.SynchronizedConditionReason, "")
	return ctrl.Result{}, nil
}

func (r *SentinelRunJobReconciler) addOrRemoveFinalizer(ctx context.Context, srj *v1alpha1.SentinelRunJob) *ctrl.Result {
	if srj.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(srj, SentinelRunJobFinalizerName) {
		controllerutil.AddFinalizer(srj, SentinelRunJobFinalizerName)
	}
	if !srj.GetDeletionTimestamp().IsZero() {
		controllerutil.RemoveFinalizer(srj, SentinelRunJobFinalizerName)
	}
	return nil
}

// SetupWithManager configures the controller with the manager.
func (r *SentinelRunJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.SentinelRunJob{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func (r *SentinelRunJobReconciler) reconcileRunJob(ctx context.Context, srj *v1alpha1.SentinelRunJob, run *console.SentinelRunJobFragment) (*batchv1.Job, error) {
	foundJob := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: srj.Name, Namespace: srj.Namespace}, foundJob); err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}

		jobSpec := getRunJobSpec(srj.Name, run.JobSpec)
		job, err := r.GenerateRunJob(run, jobSpec, srj.Name, srj.Namespace)
		if err != nil {
			return nil, err
		}
		if err := r.Create(ctx, job); err != nil {
			return nil, err
		}
		return job, nil
	}

	return foundJob, nil
}
