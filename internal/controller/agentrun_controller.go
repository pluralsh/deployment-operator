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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	pluralclient "github.com/pluralsh/deployment-operator/pkg/client"
)

const (
	AgentRunFinalizer = "deployments.plural.sh/agentrun-protection"
	AgentRunTimeout   = time.Minute * 30
)

// Mock types for development - these will be replaced with actual CRD types later
type AgentRunPhase string

const (
	AgentRunPhasePending   AgentRunPhase = "Pending"
	AgentRunPhaseRunning   AgentRunPhase = "Running"
	AgentRunPhaseSucceeded AgentRunPhase = "Succeeded"
	AgentRunPhaseFailed    AgentRunPhase = "Failed"
)

// MockAgentRun represents the structure we expect the AgentRun CRD to have
// This is a placeholder for development and will be replaced with the actual CRD
type MockAgentRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MockAgentRunSpec   `json:"spec,omitempty"`
	Status MockAgentRunStatus `json:"status,omitempty"`
}

type MockAgentRunSpec struct {
	// TODO: Define spec fields when implementing the actual CRD
	// Expected fields:
	// - AgentID string
	// - TaskType string
	// - Configuration map[string]interface{}
	// - Timeout *metav1.Duration
}

type MockAgentRunStatus struct {
	// TODO: Define status fields when implementing the actual CRD
	// Expected fields:
	Phase          AgentRunPhase           `json:"phase,omitempty"`
	StartTime      *metav1.Time            `json:"startTime,omitempty"`
	CompletionTime *metav1.Time            `json:"completionTime,omitempty"`
	PodRef         *corev1.ObjectReference `json:"podRef,omitempty"`
	Message        string                  `json:"message,omitempty"`

	// Console API integration
	ConsoleID *string `json:"consoleId,omitempty"`
}

// AgentRunReconciler is a controller for the AgentRun custom resource.
// It manages the lifecycle of individual agent runs by:
// 1. Creating pods to execute agent tasks
// 2. Monitoring pod status and updating AgentRun status
// 3. Reporting status back to Console API
type AgentRunReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ConsoleClient pluralclient.Client
}

func (r *AgentRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, retErr error) {
	agentRun := &v1alpha1.AgentRun{}
	if err := r.Get(ctx, req.NamespacedName, agentRun); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	scope, err := NewDefaultScope(ctx, r.Client, agentRun)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	utils.MarkCondition(agentRun.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "")
	utils.MarkCondition(agentRun.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReason, "")

	result := r.addOrRemoveFinalizer(ctx, agentRun)
	if result != nil {
		return *result, nil
	}

	agentRuntime, err := r.getAgentRuntime(ctx, agentRun)
	if err != nil {
		utils.MarkCondition(agentRuntime.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
		return jitterRequeue(requeueWaitForResources, jitter), nil
	}

	// TODO: Sync/upsert with Console API here? To handle manual run creation.

	if err = r.reconcilePod(ctx, agentRun, agentRuntime); err != nil {
		return ctrl.Result{}, err
	}

	return jitterRequeue(requeueAfterAgentRuntime, jitter), nil
}

func (r *AgentRunReconciler) addOrRemoveFinalizer(ctx context.Context, agentRun *v1alpha1.AgentRun) *ctrl.Result {
	if agentRun.DeletionTimestamp.IsZero() && !controllerutil.ContainsFinalizer(agentRun, AgentRuntimeFinalizer) {
		controllerutil.AddFinalizer(agentRun, AgentRuntimeFinalizer)
	}

	// If the agent runtime is being deleted, cleanup and remove the finalizer.
	if !agentRun.GetDeletionTimestamp().IsZero() {
		// If the agent runtime does not have an ID, the finalizer can be removed.
		if !agentRun.Status.HasID() {
			controllerutil.RemoveFinalizer(agentRun, AgentRuntimeFinalizer)
			return &ctrl.Result{}
		}

		exists, err := r.ConsoleClient.IsAgentRuntimeExists(ctx, agentRun.Status.GetID())
		if err != nil {
			return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
		}

		// Remove agent runtime from Console API if it exists.
		if exists {
			if err = r.ConsoleClient.DeleteAgentRuntime(ctx, agentRun.Status.GetID()); err != nil {
				// If it fails to delete the external dependency here, return with the error so that it can be retried.
				utils.MarkCondition(agentRun.SetCondition, v1alpha1.SynchronizedConditionType, metav1.ConditionFalse, v1alpha1.SynchronizedConditionReasonError, err.Error())
				return lo.ToPtr(jitterRequeue(requeueAfter, jitter))
			}
		}

		// If finalizer is present, remove it.
		controllerutil.RemoveFinalizer(agentRun, AgentRuntimeFinalizer)

		// Stop reconciliation as the item does no longer exist.
		return &ctrl.Result{}
	}

	return nil
}

func (r *AgentRunReconciler) getAgentRuntime(ctx context.Context, agentRun *v1alpha1.AgentRun) (agentRuntime *v1alpha1.AgentRuntime, err error) {
	err = r.Get(ctx, client.ObjectKey{Name: agentRun.Spec.RuntimeRef.Name}, agentRuntime)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent runtime: %w", err)
	}

	if agentRuntime == nil {
		return nil, fmt.Errorf("agent runtime %s not found", agentRun.Spec.RuntimeRef.Name)
	}

	return
}

// reconcilePod ensures the pod for the agent run exists and is in the desired state.
func (r *AgentRunReconciler) reconcilePod(ctx context.Context, agentRun *v1alpha1.AgentRun, agentRuntime *v1alpha1.AgentRuntime) error {
	var pod *corev1.Pod
	err := r.Get(ctx, client.ObjectKey{Name: agentRun.Name, Namespace: agentRun.Namespace}, pod)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        agentRun.Name,
			Namespace:   agentRun.Namespace,
			Labels:      agentRuntime.Spec.Template.Labels,
			Annotations: agentRuntime.Spec.Template.Annotations,
		},
		Spec: agentRuntime.Spec.Template.Spec,
	}

	if err = r.Create(ctx, pod); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	if err = utils.TryAddControllerRef(ctx, r.Client, agentRun, pod, r.Scheme); err != nil {
		return fmt.Errorf("failed to add controller ref: %w", err)
	}

	return nil
}

// buildPodSpec creates the pod specification for running agent tasks
// PSEUDOCODE: This will configure the execution environment
func (r *AgentRunReconciler) buildPodSpec(agentRun *MockAgentRun) corev1.PodSpec {
	// return corev1.PodSpec{
	//     RestartPolicy: corev1.RestartPolicyNever,
	//     Containers: []corev1.Container{
	//         {
	//             Name:    "agent-executor",
	//             Image:   "pluralsh/agent-harness:latest",
	//             Command: []string{"/agent-harness"},
	//             Args: []string{
	//                 "--run-id=" + agentRun.Name,
	//                 "--task-type=" + agentRun.Spec.TaskType,
	//             },
	//             Env: []corev1.EnvVar{
	//                 {Name: "CONSOLE_URL", Value: "..."},
	//                 {Name: "AGENT_ID", Value: agentRun.Spec.AgentID},
	//             },
	//             VolumeMounts: []corev1.VolumeMount{
	//                 {Name: "workspace", MountPath: "/workspace"},
	//                 {Name: "credentials", MountPath: "/credentials"},
	//             },
	//         },
	//     },
	//     Volumes: []corev1.Volume{
	//         {Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	//         {Name: "credentials", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "agent-credentials"}}},
	//     },
	// }

	return corev1.PodSpec{} // Placeholder
}

// SetupWithManager configures the controller with the manager
// PSEUDOCODE: This will be updated when the actual CRD is available
func (r *AgentRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.AgentRun{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}).
		Complete(r)
}
