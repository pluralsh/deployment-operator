package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	globalName       = "global"
	debounceDuration = time.Second * 30
)

var supportedMetricsAPIVersions = []string{
	"v1beta1",
}

// MetricsAggregateReconciler reconciles a MetricsAggregate resource.
type MetricsAggregateReconciler struct {
	k8sClient.Client
	Scheme          *runtime.Scheme
	DiscoveryClient discovery.DiscoveryInterface
}

// Reconcile IngressReplica ensure that stays in sync with Kubernetes cluster.
func (r *MetricsAggregateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	apiGroups, err := r.DiscoveryClient.ServerGroups()
	if err != nil {
		return reconcile.Result{}, err
	}
	metricsAPIAvailable := SupportedMetricsAPIVersionAvailable(apiGroups)
	if !metricsAPIAvailable {
		logger.V(5).Info("metrics api not available")
		return requeue(time.Minute*5, jitter), nil
	}

	// Read resource from Kubernetes cluster.
	metrics := &v1alpha1.MetricsAggregate{}
	if err := r.Get(ctx, req.NamespacedName, metrics); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.initGlobalMetricsAggregate(ctx); err != nil {
				return ctrl.Result{}, err
			}
			return requeue(time.Second, jitter), nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("reconciling MetricsAggregate", "namespace", metrics.Namespace, "name", metrics.Name)
	utils.MarkCondition(metrics.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "")

	scope, err := NewDefaultScope(ctx, r.Client, metrics)
	if err != nil {
		logger.Error(err, "failed to create scope")
		utils.MarkCondition(metrics.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !metrics.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return reconcile.Result{}, err
	}

	availableResources := make(map[string]corev1.ResourceList)
	for _, n := range nodeList.Items {
		availableResources[n.Name] = n.Status.Allocatable
	}

	nodeDeploymentNodesMetrics := make([]v1beta1.NodeMetrics, 0)
	allNodeMetricsList := &v1beta1.NodeMetricsList{}
	if err := r.List(ctx, allNodeMetricsList); err != nil {
		return reconcile.Result{}, err
	}

	for _, m := range allNodeMetricsList.Items {
		if _, ok := availableResources[m.Name]; ok {
			nodeDeploymentNodesMetrics = append(nodeDeploymentNodesMetrics, m)
		}
	}

	nodeMetrics, err := ConvertNodeMetrics(nodeDeploymentNodesMetrics, availableResources)
	if err != nil {
		return reconcile.Result{}, err
	}

	// save metrics
	metrics.Status.Nodes = len(nodeList.Items)
	for _, nm := range nodeMetrics {
		metrics.Status.CPUAvailableMillicores += nm.CPUAvailableMillicores
		metrics.Status.CPUTotalMillicores += nm.CPUTotalMillicores
		metrics.Status.MemoryAvailableBytes += nm.MemoryAvailableBytes
		metrics.Status.MemoryTotalBytes += nm.MemoryTotalBytes
	}

	fraction := float64(metrics.Status.CPUTotalMillicores) / float64(metrics.Status.CPUAvailableMillicores) * 100
	metrics.Status.CPUUsedPercentage = int64(fraction)
	fraction = float64(metrics.Status.MemoryTotalBytes) / float64(metrics.Status.MemoryAvailableBytes) * 100
	metrics.Status.MemoryUsedPercentage = int64(fraction)

	utils.MarkCondition(metrics.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionTrue, v1alpha1.ReadyConditionReason, "")

	return requeue(requeueAfter, jitter), reterr
}

// SetupWithManager sets up the controller with the Manager.
func (r *MetricsAggregateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	debounceReconciler := NewDebounceReconciler(mgr.GetClient(), debounceDuration, r)
	debounceReconciler.Start(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MetricsAggregate{}).
		Complete(debounceReconciler)
}

func (r *MetricsAggregateReconciler) initGlobalMetricsAggregate(ctx context.Context) error {
	// Init global MetricsAggregate object
	if err := r.Get(ctx, k8sClient.ObjectKey{Name: globalName}, &v1alpha1.MetricsAggregate{}); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, &v1alpha1.MetricsAggregate{
				ObjectMeta: metav1.ObjectMeta{
					Name: globalName,
				},
			}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

type ResourceMetricsInfo struct {
	Name      string
	Metrics   corev1.ResourceList
	Available corev1.ResourceList
}

func ConvertNodeMetrics(metrics []v1beta1.NodeMetrics, availableResources map[string]corev1.ResourceList) ([]v1alpha1.MetricsAggregateStatus, error) {
	nodeMetrics := make([]v1alpha1.MetricsAggregateStatus, 0)

	if metrics == nil {
		return nil, fmt.Errorf("metric list can not be nil")
	}

	for _, m := range metrics {
		nodeMetric := v1alpha1.MetricsAggregateStatus{}

		resourceMetricsInfo := ResourceMetricsInfo{
			Name:      m.Name,
			Metrics:   m.Usage.DeepCopy(),
			Available: availableResources[m.Name],
		}

		if available, found := resourceMetricsInfo.Available[corev1.ResourceCPU]; found {
			quantityCPU := resourceMetricsInfo.Metrics[corev1.ResourceCPU]
			// cpu in mili cores
			nodeMetric.CPUTotalMillicores = quantityCPU.MilliValue()
			nodeMetric.CPUAvailableMillicores = available.MilliValue()
		}

		if available, found := resourceMetricsInfo.Available[corev1.ResourceMemory]; found {
			quantityM := resourceMetricsInfo.Metrics[corev1.ResourceMemory]
			// memory in bytes
			nodeMetric.MemoryTotalBytes = quantityM.Value() / (1024 * 1024)
			nodeMetric.MemoryAvailableBytes = available.Value() / (1024 * 1024)
		}
		nodeMetrics = append(nodeMetrics, nodeMetric)
	}

	return nodeMetrics, nil
}

func SupportedMetricsAPIVersionAvailable(discoveredAPIGroups *metav1.APIGroupList) bool {
	for _, discoveredAPIGroup := range discoveredAPIGroups.Groups {
		if discoveredAPIGroup.Name != metricsapi.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			for _, supportedVersion := range supportedMetricsAPIVersions {
				if version.Version == supportedVersion {
					return true
				}
			}
		}
	}
	return false
}
