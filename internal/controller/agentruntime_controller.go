package controller

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
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
	ConsoleClient consoleclient.Client
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
		apiAgentRuntime, err := r.ConsoleClient.UpsertAgentRuntime(ctx, agentRuntime.Attributes())
		if err != nil {
			return handleRequeue(nil, err, agentRuntime.SetCondition)
		}

		agentRuntime.Status.ID = &apiAgentRuntime.ID
	}
	agentRuntime.Status.SHA = &sha

	// Mark as synchronized after the agent runtime is synchronized with the Console API.
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionTrue, v1alpha1.SynchronizedConditionReason, "")

	var errors []error
	pager := r.ListAgentRuntimePendingRuns(ctx, agentRuntime.Status.GetID())
	for pager.HasNext() {
		runs, err := pager.NextPage()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to fetch agent runtime pending runs: %w", err)
		}

		for _, run := range runs {
			if err := r.createAgentRun(ctx, agentRuntime, run.Node); err != nil {
				logger.Error(err, "failed to create agent run", "id", run.Node.ID)
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		aggregateError := utilerrors.NewAggregate(errors)
		utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReasonError, aggregateError.Error())
		return jitterRequeue(requeueAfterAgentRuntime, jitter), nil
	}

	// Mark as ready after the agent run custom resources are created.
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")

	return jitterRequeue(requeueAfterAgentRuntime, jitter), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentRuntimeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.AgentRuntime{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&v1alpha1.AgentRun{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(r)
}

func (r *AgentRuntimeReconciler) ListAgentRuntimePendingRuns(ctx context.Context, id string) *algorithms.Pager[*console.ListAgentRuntimePendingRuns_AgentRuntime_PendingRuns_Edges] {
	logger := log.FromContext(ctx)
	fetch := func(page *string, size int64) ([]*console.ListAgentRuntimePendingRuns_AgentRuntime_PendingRuns_Edges, *algorithms.PageInfo, error) {
		resp, err := r.ConsoleClient.ListAgentRuntimePendingRuns(ctx, id, page, &size)
		if err != nil {
			logger.Error(err, "failed to fetch stack run")
			return nil, nil, err
		}
		pageInfo := &algorithms.PageInfo{
			HasNext:  resp.PageInfo.HasNextPage,
			After:    resp.PageInfo.EndCursor,
			PageSize: size,
		}
		return resp.Edges, pageInfo, nil
	}
	return algorithms.NewPager[*console.ListAgentRuntimePendingRuns_AgentRuntime_PendingRuns_Edges](100, fetch)
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

		exists, err := r.ConsoleClient.IsAgentRuntimeExists(ctx, agentRuntime.Status.GetID())
		if err != nil {
			return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
		}

		// Remove agent runtime from Console API if it exists.
		if exists {
			if err = r.ConsoleClient.DeleteAgentRuntime(ctx, agentRuntime.Status.GetID()); err != nil {
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

func (r *AgentRuntimeReconciler) createAgentRun(ctx context.Context, agentRuntime *v1alpha1.AgentRuntime, run *console.AgentRunFragment) error {
	logger := log.FromContext(ctx)

	if err := r.Get(ctx, client.ObjectKey{Name: run.ID, Namespace: agentRuntime.Spec.TargetNamespace}, &v1alpha1.AgentRun{}); err == nil {
		logger.V(4).Info("agent run already exists",
			"name", run.ID, "namespace", agentRuntime.Spec.TargetNamespace)
		return nil
	}

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.ID,
			Namespace: agentRuntime.Spec.TargetNamespace,
			Labels:    map[string]string{"deployments.plural.sh/agent-runtime": agentRuntime.ConsoleName()},
		},
		Spec: v1alpha1.AgentRunSpec{
			Prompt:     run.Prompt,
			Repository: run.Repository,
			Mode:       run.Mode,
		},
	}
	if run.Flow != nil {
		agentRun.Spec.FlowID = lo.ToPtr(run.Flow.ID)
	}

	if err := r.ensureNamespace(ctx, agentRuntime.Spec.TargetNamespace); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	if err := r.Create(ctx, agentRun); err != nil {
		return fmt.Errorf("failed to create agent run: %w", err)
	}

	if err := utils.TryAddControllerRef(ctx, r.Client, agentRuntime, agentRun, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference for agent run: %w", err)
	}

	return nil
}

func (r *AgentRuntimeReconciler) ensureNamespace(ctx context.Context, namespace string) error {
	if namespace == "" {
		return nil
	}
	if err := r.Get(ctx, client.ObjectKey{Name: namespace}, &corev1.Namespace{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return r.Create(ctx, &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: namespace,
			},
		})
	}
	return nil
}
