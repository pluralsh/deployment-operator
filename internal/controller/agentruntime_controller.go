package controller

import (
	"context"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
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
	_ = log.FromContext(ctx)
	agentRuntime := &v1alpha1.AgentRuntime{}
	if err := r.Get(ctx, req.NamespacedName, agentRuntime); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.SynchronizedConditionReason, "")

	scope, err := NewDefaultScope(ctx, r.Client, agentRuntime)
	if err != nil {
		utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if !agentRuntime.GetDeletionTimestamp().IsZero() {
		return r.handleDelete(ctx, agentRuntime)
	}

	return ctrl.Result{RequeueAfter: requeueAfterAgentRuntime}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentRuntimeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.AgentRuntime{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

func (r *AgentRuntimeReconciler) handleDelete(ctx context.Context, agentRuntime *v1alpha1.AgentRuntime) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if controllerutil.ContainsFinalizer(agentRuntime, AgentRuntimeFinalizer) {
		if agentRuntime.Status.GetID() != "" {
			if _, err := r.consoleClient.GetAgentRuntime(ctx, agentRuntime.Status.GetID()); err != nil {
				if !errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				controllerutil.RemoveFinalizer(agentRuntime, AgentRuntimeFinalizer)
				return ctrl.Result{}, nil
			}
			if err := r.consoleClient.DeleteAgentRuntime(ctx, agentRuntime.Status.GetID()); err != nil {
				return ctrl.Result{}, err
			}
		}

		controllerutil.RemoveFinalizer(agentRuntime, AgentRuntimeFinalizer)
		logger.Info("agent runtime deleted successfully")
	}

	return ctrl.Result{}, nil
}
