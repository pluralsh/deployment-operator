package controller

import (
	"context"
	"time"

	console "github.com/pluralsh/console/go/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/polly/algorithms"
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

	pager := r.ListAgentRuns(ctx)
	for pager.HasNext() {
		runs, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch run list")
			return ctrl.Result{}, err
		}
		for _, run := range runs {
			if run.Node.Status == console.AgentRunStatusPending {
				// Create Agent CRD for this pending run to be picked up by agentrun controller
				if err := r.createAgentRun(ctx, agentRuntime, run.Node); err != nil {

				}
			}
		}
	}

	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, v1.ConditionTrue, v1alpha1.SynchronizedConditionReason, "")

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

func (r *AgentRuntimeReconciler) ListAgentRuns(ctx context.Context) *algorithms.Pager[*console.ListAgentRuns_AgentRuns_Edges] {
	logger := log.FromContext(ctx)
	logger.V(4).Info("create pager")
	fetch := func(page *string, size int64) ([]*console.ListAgentRuns_AgentRuns_Edges, *algorithms.PageInfo, error) {
		resp, err := r.consoleClient.ListAgentRuns(ctx, page, &size)
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
	return algorithms.NewPager[*console.ListAgentRuns_AgentRuns_Edges](100, fetch)
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

func (r *AgentRuntimeReconciler) createAgentRun(ctx context.Context, agentRuntime *v1alpha1.AgentRuntime, run *console.AgentRunFragment) error {
	logger := log.FromContext(ctx)
	if err := r.Get(ctx, client.ObjectKey{Name: run.ID, Namespace: agentRuntime.Namespace}, &v1alpha1.AgentRun{}); err == nil {
		logger.Info("AgentRun already exists", "runID", run.ID)
		return nil
	}

	agentRun := &v1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.ID,
			Namespace: agentRuntime.Namespace,
			Labels: map[string]string{
				"deployments.plural.sh/agent-runtime": agentRuntime.ConsoleName(),
			},
		},
		Spec: v1alpha1.AgentRunSpec{
			Prompt:     run.Prompt,
			Repository: run.Repository,
			Mode:       run.Mode,
			FlowID:     &run.Flow.ID,
		},
	}

	if err := r.Create(ctx, agentRun); err != nil {
		logger.Error(err, "failed to create AgentRun CRD", "runID", run.ID)
		return err
	}
	if err := utils.TryAddControllerRef(ctx, r.Client, agentRuntime, agentRun, r.Scheme); err != nil {
		logger.Error(err, "Error setting ControllerReference for AgentRun.")
		return err
	}

	return nil
}
