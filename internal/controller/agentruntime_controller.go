package controller

import (
	"context"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	AgentRuntimeFinalizer    = "deployments.plural.sh/agent-runtime-protection"
	requeueAfterAgentRuntime = 2 * time.Minute
)

// AgentRuntimeReconciler reconciles a AgentRuntime object
type AgentRuntimeReconciler struct {
	client.Client
	consoleClient consoleclient.Client
	Scheme        *runtime.Scheme
}

//+kubebuilder:rbac:groups=deployments.plural.sh,resources=agentruntimes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=agentruntimes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=agentruntimes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *AgentRuntimeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)

	agentRuntime := &v1alpha1.AgentRuntime{}
	if err := r.Get(ctx, req.NamespacedName, agentRuntime); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scope, err := NewDefaultScope(ctx, r.Client, agentRuntime)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.SynchronizedConditionReason, "")

	result := r.addOrRemoveFinalizer(ctx, agentRuntime)
	if result != nil {
		return *result, nil
	}

	changed, sha, err := agentRuntime.Diff(utils.HashObject)
	if err != nil {
		logger.Error(err, "unable to calculate agent runtime SHA")
		utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
		return ctrl.Result{}, err
	}

	if changed {
		apiAgentRuntime, err := r.consoleClient.UpsertAgentRuntime(ctx, agentRuntime.Attributes())
		if err != nil {
			return handleRequeue(nil, err, agentRuntime.SetCondition)
		}

		agentRuntime.Status.ID = &apiAgentRuntime.ID
	}

	agentRuntime.Status.SHA = &sha

	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionTrue, v1alpha1.SynchronizedConditionReason, "")

	return jitterRequeue(requeueAfterAgentRuntime, jitter), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentRuntimeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.AgentRuntime{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func (r *AgentRuntimeReconciler) addOrRemoveFinalizer(ctx context.Context, agentRuntime *v1alpha1.AgentRuntime) *ctrl.Result {
	if agentRuntime.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(agentRuntime, AgentRuntimeFinalizer) {
		controllerutil.AddFinalizer(agentRuntime, AgentRuntimeFinalizer)
	}

	// If the agent runtime is being deleted, cleanup and remove the finalizer.
	if !agentRuntime.GetDeletionTimestamp().IsZero() {
		// If the agent runtime does not have an ID, the finalizer can be removed.
		if !agentRuntime.Status.HasID() {
			controllerutil.RemoveFinalizer(agentRuntime, AgentRuntimeFinalizer)
			return &ctrl.Result{}
		}

		exists, err := r.consoleClient.IsAgentRuntimeExists(ctx, agentRuntime.Status.GetID())
		if err != nil {
			return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
		}

		// Remove agent runtime from Console API if it exists.
		if exists {
			if err = r.consoleClient.DeleteAgentRuntime(ctx, agentRuntime.Status.GetID()); err != nil {
				// If it fails to delete the external dependency here, return with the error so that it can be retried.
				utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
				return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
			}
		}

		// If finalizer is present, remove it.
		controllerutil.RemoveFinalizer(agentRuntime, AgentRuntimeFinalizer)

		// Stop reconciliation as the item does no longer exist.
		return &ctrl.Result{}
	}

	return nil
}
