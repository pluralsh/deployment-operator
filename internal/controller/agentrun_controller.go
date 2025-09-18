package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	pluralclient "github.com/pluralsh/deployment-operator/pkg/client"
)

const (
	AgentRunFinalizer    = "deployments.plural.sh/agentrun-protection"
	requeueAfterAgentRun = 2 * time.Minute
	envConsoleURL        = "PLRL_CONSOLE_URL"
	envConsoleToken      = "PLRL_CONSOLE_TOKEN"
	envAgentRunID        = "PLRL_AGENT_RUN_ID"
)

// AgentRunReconciler is a controller for the AgentRun custom resource.
// It manages the lifecycle of individual agent runs by:
// 1. Creating pods to execute agent tasks
// 2. Monitoring pod status and updating AgentRun status
// 3. Reporting status back to Console API
type AgentRunReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ConsoleClient pluralclient.Client
	ConsoleURL    string
	ConsoleToken  string
}

// SetupWithManager configures the controller with the manager.
func (r *AgentRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.AgentRun{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func (r *AgentRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, retErr error) {
	run := &v1alpha1.AgentRun{}
	if err := r.Get(ctx, req.NamespacedName, run); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scope, err := NewDefaultScope(ctx, r.Client, run)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	utils.MarkCondition(run.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(run.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReason, "")

	result := r.addOrRemoveFinalizer(ctx, run)
	if result != nil {
		return *result, nil
	}

	runtime, err := r.getRuntime(ctx, run)
	if err != nil {
		utils.MarkCondition(runtime.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
		return jitterRequeue(requeueWaitForResources, jitter), nil
	}

	// TODO: Sync/upsert with Console API here? To handle manual run creation.

	if err = r.reconcilePod(ctx, run, runtime); err != nil {
		return ctrl.Result{}, err
	}

	return jitterRequeue(requeueAfterAgentRun, jitter), nil
}

func (r *AgentRunReconciler) addOrRemoveFinalizer(ctx context.Context, run *v1alpha1.AgentRun) *ctrl.Result {
	if run.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(run, AgentRuntimeFinalizer) {
		controllerutil.AddFinalizer(run, AgentRunFinalizer)
	}

	// If the agent run is being deleted, cleanup and remove the finalizer.
	if !run.GetDeletionTimestamp().IsZero() {
		// If the agent run does not have an ID, the finalizer can be removed.
		if !run.Status.HasID() {
			controllerutil.RemoveFinalizer(run, AgentRunFinalizer)
			return &ctrl.Result{}
		}

		exists, err := r.ConsoleClient.IsAgentRunExists(ctx, run.Status.GetID())
		if err != nil {
			return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
		}

		// Cancel agent run from Console API if it exists.
		if exists {
			if err = r.ConsoleClient.CancelAgentRun(ctx, run.Status.GetID()); err != nil {
				// If it fails to delete the external dependency here, return with the error so that it can be retried.
				utils.MarkCondition(run.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
				return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
			}
		}

		// If finalizer is present, remove it.
		controllerutil.RemoveFinalizer(run, AgentRunFinalizer)

		// Stop reconciliation as the item does no longer exist.
		return &ctrl.Result{}
	}

	return nil
}

func (r *AgentRunReconciler) getRuntime(ctx context.Context, run *v1alpha1.AgentRun) (runtime *v1alpha1.AgentRuntime, err error) {
	runtime = &v1alpha1.AgentRuntime{}
	err = r.Get(ctx, client.ObjectKey{Name: run.Spec.RuntimeRef.Name}, runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent runtime: %w", err)
	}

	if runtime == nil {
		return nil, fmt.Errorf("agent runtime %s not found", run.Spec.RuntimeRef.Name)
	}

	return
}

// reconcilePod ensures the pod for the agent run exists and is in the desired state.
func (r *AgentRunReconciler) reconcilePod(ctx context.Context, run *v1alpha1.AgentRun, runtime *v1alpha1.AgentRuntime) error {
	secret, err := r.reconcilePodSecret(ctx, run)
	if err != nil {
		return fmt.Errorf("failed to reconcile run secret: %w", err)
	}

	pod := &corev1.Pod{}
	err = r.Get(ctx, client.ObjectKey{Name: run.Name, Namespace: run.Namespace}, pod)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	pod = buildAgentRunPod(run, runtime)
	if err = r.Create(ctx, pod); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	if err = utils.TryAddControllerRef(ctx, r.Client, run, pod, r.Scheme); err != nil {
		return fmt.Errorf("failed to add controller ref: %w", err)
	}

	if err := utils.TryAddOwnerRef(ctx, r.Client, pod, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to add owner ref: %w", err)
	}

	return nil
}

func (r *AgentRunReconciler) reconcilePodSecret(ctx context.Context, run *v1alpha1.AgentRun) (*corev1.Secret, error) {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: run.Name, Namespace: run.Namespace}, secret); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get secret: %w", err)
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: run.Name, Namespace: run.Namespace},
			StringData: r.getSecretData(run),
		}

		logger.V(2).Info("creating secret", "namespace", secret.Namespace, "name", secret.Name)
		if err = r.Create(ctx, secret); err != nil {
			return nil, fmt.Errorf("failed to create secret: %w", err)
		}

		return secret, nil
	}

	if !r.hasSecretData(secret.Data, run) {
		logger.V(2).Info("updating secret", "namespace", secret.Namespace, "name", secret.Name)
		secret.StringData = r.getSecretData(run)
		if err := r.Update(ctx, secret); err != nil {
			logger.Error(err, "unable to update secret")
			return nil, err
		}
	}

	return secret, nil
}

func (r *AgentRunReconciler) getSecretData(run *v1alpha1.AgentRun) map[string]string {
	return map[string]string{
		envConsoleURL:   r.ConsoleURL,
		envConsoleToken: r.ConsoleToken,
		envAgentRunID:   run.Status.GetID(),
	}
}

func (r *AgentRunReconciler) hasSecretData(data map[string][]byte, run *v1alpha1.AgentRun) bool {
	token, hasToken := data[envConsoleToken]
	url, hasUrl := data[envConsoleURL]
	id, hasID := data[envAgentRunID]
	return hasToken && hasUrl && hasID &&
		string(token) == r.ConsoleToken && string(url) == r.ConsoleURL && string(id) == run.Status.GetID()
}
