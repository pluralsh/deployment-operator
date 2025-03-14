package controller

import (
	"context"
	"fmt"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
	"strconv"
)

const defaultBatchSize = 50

// ClusterDrainReconciler reconciles a ClusterDrain object
type ClusterDrainReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile executes the drain logic once per ClusterDrain object
func (r *ClusterDrainReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	// Fetch the ClusterDrain object
	drain := &v1alpha1.ClusterDrain{}
	if err := r.Get(ctx, req.NamespacedName, drain); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ensure that status updates will always be persisted when exiting this function.
	scope, err := NewDefaultScope(ctx, r.Client, drain)
	if err != nil {
		logger.Error(err, "Failed to create drain scope")
		utils.MarkCondition(drain.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}
	defer func() {
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if meta.IsStatusConditionTrue(drain.Status.Conditions, v1alpha1.ReadyConditionType.String()) {
		// Do not requeue; execute once per CR instance
		return ctrl.Result{}, nil
	}

	// Fetch workloads matching labelSelector
	workloads, err := r.getMatchingWorkloads(ctx, drain)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Sort workloads by wave, then namespace/name
	sortWorkloads(workloads)

	// Apply drain logic
	progress, err := r.applyDrain(ctx, drain, workloads)
	if err != nil {
		utils.MarkCondition(drain.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	drain.Status.Progress = progress
	utils.MarkCondition(drain.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")

	return ctrl.Result{}, nil
}

// applyDrain annotates workloads in waves, respecting flow control
func (r *ClusterDrainReconciler) applyDrain(ctx context.Context, drain *v1alpha1.ClusterDrain, workloads []unstructured.Unstructured) ([]v1alpha1.Progress, error) {
	progress := []v1alpha1.Progress{}
	var batchSize int
	if drain.Spec.FlowControl.Percentage != nil {
		batchSize = *drain.Spec.FlowControl.Percentage * len(workloads) / 100
	}
	if drain.Spec.FlowControl.MaxConcurrency != nil {
		batchSize = *drain.Spec.FlowControl.MaxConcurrency
	}
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}

	waves := splitIntoWaves(workloads, batchSize)

	var failed []corev1.ObjectReference
	for i, wave := range waves {
		for _, obj := range wave {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations["deployments.plural.sh/drain-wave"] = strconv.Itoa(i)
			obj.SetAnnotations(annotations)

			// Extract and modify PodTemplateSpec annotations
			annotations, found, err := unstructured.NestedStringMap(obj.Object, "spec", "template", "metadata", "annotations")
			if err != nil {
				return nil, fmt.Errorf("failed to get annotations: %w", err)
			}

			if !found {
				annotations = make(map[string]string)
			}
			annotations["deployments.plural.sh/drain"] = drain.Name

			// Set the modified annotations back into the object
			err = unstructured.SetNestedStringMap(obj.Object, annotations, "spec", "template", "metadata", "annotations")
			if err != nil {
				return nil, fmt.Errorf("failed to set annotations: %w", err)
			}

			if err := r.Update(ctx, &obj); err != nil {
				failed = append(failed, corev1.ObjectReference{
					APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
					Name:       obj.GetName(),
					Namespace:  obj.GetNamespace(),
				})
			}
		}

		progress = append(progress, v1alpha1.Progress{
			Wave:       i,
			Percentage: len(wave) * 100 / len(workloads),
			Count:      len(wave),
			Failures:   failed,
		})

	}

	return progress, nil
}

func splitIntoWaves[T any](items []T, batchSize int) [][]T {
	var result [][]T
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items) // Handle the last batch if it has fewer items
		}
		result = append(result, items[i:end])
	}
	return result
}

// SetupWithManager registers the controller
func (r *ClusterDrainReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterDrain{}).
		Complete(r)
}

// getMatchingWorkloads fetches Deployments, StatefulSets, DaemonSets that match the label selector
func (r *ClusterDrainReconciler) getMatchingWorkloads(ctx context.Context, drain *v1alpha1.ClusterDrain) ([]unstructured.Unstructured, error) {
	var allWorkloads []unstructured.Unstructured

	// Define selectors
	selector, err := metav1.LabelSelectorAsSelector(drain.Spec.LabelSelector)
	if err != nil {
		return nil, err
	}

	// Fetch workloads
	workloadTypes := []schema.GroupVersionKind{
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Group: "apps", Version: "v1", Kind: "DaemonSet"},
		{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	}

	for _, gvk := range workloadTypes {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		if err := r.List(ctx, list, &client.ListOptions{LabelSelector: selector}); err != nil {
			return nil, err
		}
		allWorkloads = append(allWorkloads, list.Items...)
	}

	return allWorkloads, nil
}

// sortWorkloads sorts workloads by wave first, then namespace/name
func sortWorkloads(workloads []unstructured.Unstructured) {
	sort.Slice(workloads, func(i, j int) bool {
		waveI := getWave(workloads[i])
		waveJ := getWave(workloads[j])
		if waveI != waveJ {
			return waveI < waveJ
		}
		return workloads[i].GetNamespace()+workloads[i].GetName() < workloads[j].GetNamespace()+workloads[j].GetName()
	})
}

// getWave extracts wave number from annotations
func getWave(obj unstructured.Unstructured) int {
	if val, ok := obj.GetAnnotations()["deployments.plural.sh/drain-wave"]; ok {
		var wave int
		_, err := fmt.Sscanf(val, "%d", &wave)
		if err != nil {
			return 0
		}
		return wave
	}
	return 0
}
