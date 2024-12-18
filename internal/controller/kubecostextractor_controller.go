/*
Copyright 2024.

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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opencost/opencost/core/pkg/opencost"
	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var kubecostResourceTypes = []string{"deployment", "statefulset", "daemonset"}

// KubecostExtractorReconciler reconciles a KubecostExtractor object
type KubecostExtractorReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	KubeClient       kubernetes.Interface
	ExtConsoleClient consoleclient.Client
	Tasks            cmap.ConcurrentMap[string, context.CancelFunc]
}

func (r *KubecostExtractorReconciler) RunOnInterval(ctx context.Context, key string, interval time.Duration, condition wait.ConditionWithContextFunc) {
	if _, exists := r.Tasks.Get(key); exists {
		return
	}
	ctxCancel, cancel := context.WithCancel(ctx)
	r.Tasks.Set(key, cancel)

	go func() {
		_ = wait.PollUntilContextCancel(ctxCancel, interval, true, condition)
	}()
}

//+kubebuilder:rbac:groups=deployments.plural.sh,resources=kubecostextractors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=kubecostextractors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployments.plural.sh,resources=kubecostextractors/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *KubecostExtractorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	kubecost := &v1alpha1.KubecostExtractor{}
	if err := r.Get(ctx, req.NamespacedName, kubecost); err != nil {
		logger.Error(err, "Unable to fetch kubecost")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !kubecost.DeletionTimestamp.IsZero() {
		if cancel, exists := r.Tasks.Get(req.NamespacedName.String()); exists {
			cancel()
			r.Tasks.Remove(req.NamespacedName.String())
		}
	}

	utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, "")

	scope, err := NewDefaultScope(ctx, r.Client, kubecost)
	if err != nil {
		logger.Error(err, "failed to create scope")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// check service
	kubecostService := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: kubecost.Spec.KubecostServiceRef.Name, Namespace: kubecost.Spec.KubecostServiceRef.Namespace}, kubecostService); err != nil {
		logger.Error(err, "Unable to fetch service for kubecost")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
		return ctrl.Result{}, err
	}
	recommendationThreshold, err := strconv.ParseFloat(kubecost.Spec.RecommendationThreshold, 64)
	if err != nil {
		logger.Error(err, "Unable to parse recommendation threshold")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	r.RunOnInterval(ctx, req.NamespacedName.String(), kubecost.Spec.GetInterval(), func(ctx context.Context) (done bool, err error) {
		// Always patch object when exiting this function, so we can persist any object changes.
		defer func() {
			if err := scope.PatchObject(); err != nil && reterr == nil {
				reterr = err
			}
		}()
		recommendations, err := r.getRecommendationAttributes(ctx, kubecostService, kubecost.Spec.GetPort(), kubecost.Spec.GetInterval(), recommendationThreshold)
		if err != nil {
			logger.Error(err, "Unable to fetch recommendations")
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
			return false, nil
		}
		clusterCostAttr, err := r.getClusterCost(ctx, kubecostService, kubecost.Spec.GetPort(), kubecost.Spec.GetInterval())
		if err != nil {
			logger.Error(err, "Unable to fetch cluster cost")
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
			return false, nil
		}
		namespacesCostAtrr, err := r.getNamespacesCost(ctx, kubecostService, kubecost.Spec.GetPort(), kubecost.Spec.GetInterval())
		if err != nil {
			logger.Error(err, "Unable to fetch namespacesCostAtrr cost")
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
			return false, nil
		}

		// nothing for specified time window
		if clusterCostAttr == nil && namespacesCostAtrr == nil && recommendations == nil {
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
			return false, nil
		}

		if _, err := r.ExtConsoleClient.IngestClusterCost(console.CostIngestAttributes{
			Cluster:         clusterCostAttr,
			Namespaces:      namespacesCostAtrr,
			Recommendations: recommendations,
		}); err != nil {
			logger.Error(err, "Unable to ingest cluster cost")
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
			return false, nil
		}
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
		return false, nil
	})

	return ctrl.Result{}, nil
}

func (r *KubecostExtractorReconciler) getAllocation(ctx context.Context, srv *corev1.Service, servicePort, aggregate string) (*allocationResponse, error) {
	now := time.Now()
	oneMonthBefore := now.AddDate(0, -1, 0) // 0 years, -1 month, 0 days
	window := fmt.Sprintf("%d,%d", oneMonthBefore.Unix(), now.Unix())

	queryParams := map[string]string{
		"window":     window,
		"aggregate":  aggregate,
		"accumulate": "true",
	}

	bytes, err := r.KubeClient.CoreV1().Services(srv.Namespace).ProxyGet("", srv.Name, servicePort, "/model/allocation", queryParams).DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	ar := &allocationResponse{}
	if err = json.Unmarshal(bytes, ar); err != nil {
		return nil, err
	}
	return ar, nil
}

func (r *KubecostExtractorReconciler) getRecommendationAttributes(ctx context.Context, srv *corev1.Service, servicePort string, interval time.Duration, recommendationThreshold float64) ([]*console.ClusterRecommendationAttributes, error) {
	var result []*console.ClusterRecommendationAttributes
	for _, resourceType := range kubecostResourceTypes {
		ar, err := r.getAllocation(ctx, srv, servicePort, resourceType)
		if err != nil {
			return nil, err
		}
		if ar.Code != http.StatusOK {
			return nil, fmt.Errorf("unexpected status code: %d", ar.Code)
		}
		for _, resourceCosts := range ar.Data {
			if resourceCosts == nil {
				continue
			}
			for name, allocation := range resourceCosts {
				if name == opencost.IdleSuffix || name == opencost.UnallocatedSuffix {
					continue
				}
				totalCost := allocation.TotalCost()
				if totalCost > recommendationThreshold {
					result = append(result, r.convertClusterRecommendationAttributes(ctx, allocation, name, resourceType))
				}
			}
		}
	}

	return result, nil
}

func (r *KubecostExtractorReconciler) getNamespacesCost(ctx context.Context, srv *corev1.Service, servicePort string, interval time.Duration) ([]*console.CostAttributes, error) {
	var result []*console.CostAttributes
	ar, err := r.getAllocation(ctx, srv, servicePort, "namespace")
	if err != nil {
		return nil, err
	}
	if ar.Code != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", ar.Code)
	}
	for _, clusterCosts := range ar.Data {
		if clusterCosts == nil {
			continue
		}
		for namespace, allocation := range clusterCosts {
			if namespace == opencost.IdleSuffix {
				continue
			}
			attr := convertCostAttributes(allocation)
			attr.Namespace = lo.ToPtr(namespace)
			result = append(result, attr)
		}
	}

	return result, nil
}

func (r *KubecostExtractorReconciler) getClusterCost(ctx context.Context, srv *corev1.Service, servicePort string, interval time.Duration) (*console.CostAttributes, error) {
	bytes, err := r.KubeClient.CoreV1().Services(srv.Namespace).ProxyGet("", srv.Name, servicePort, "/model/clusterInfo", nil).DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var resp clusterinfoResponse
	err = json.Unmarshal(bytes, &resp)
	if err != nil {
		return nil, err
	}

	ar, err := r.getAllocation(ctx, srv, servicePort, "cluster")
	if err != nil {
		return nil, err
	}
	if ar.Code != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", ar.Code)
	}
	for _, clusterCosts := range ar.Data {
		if clusterCosts == nil {
			continue
		}
		clusterCostAllocation, ok := clusterCosts[resp.Data.ClusterID]
		if ok {
			return convertCostAttributes(clusterCostAllocation), nil
		}
	}

	return nil, nil
}

func (r *KubecostExtractorReconciler) getObjectInfo(ctx context.Context, resourceType console.ScalingRecommendationType, namespace, name string) (container, serviceId *string, err error) {
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
	}
	switch resourceType {
	case console.ScalingRecommendationTypeDeployment:
		gvk.Kind = "Deployment"
	case console.ScalingRecommendationTypeDaemonset:
		gvk.Kind = "DaemonSet"
	case console.ScalingRecommendationTypeStatefulset:
		gvk.Kind = "StatefulSet"
	default:
		return nil, nil, nil
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	if err = r.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj); err != nil {
		return
	}
	svcId, ok := obj.GetAnnotations()[inventory.OwningInventoryKey]
	if ok {
		serviceId = lo.ToPtr(svcId)
	}

	return
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubecostExtractorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubecostExtractor{}).
		Complete(r)
}

type allocationResponse struct {
	Code int                              `json:"code"`
	Data []map[string]opencost.Allocation `json:"data"`
}

type clusterinfoResponse struct {
	Data struct {
		ClusterID string `json:"id"`
	} `json:"data"`
}

func (r *KubecostExtractorReconciler) convertClusterRecommendationAttributes(ctx context.Context, allocation opencost.Allocation, name, resourceType string) *console.ClusterRecommendationAttributes {
	resourceTypeEnum := console.ScalingRecommendationType(strings.ToUpper(resourceType))
	result := &console.ClusterRecommendationAttributes{
		Type:          lo.ToPtr(resourceTypeEnum),
		Name:          lo.ToPtr(name),
		MemoryRequest: lo.ToPtr(allocation.RAMBytesRequestAverage),
		CPURequest:    lo.ToPtr(allocation.CPUCoreRequestAverage),
		CPUCost:       lo.ToPtr(allocation.CPUCost),
		MemoryCost:    lo.ToPtr(allocation.RAMCost),
		GpuCost:       lo.ToPtr(allocation.GPUCost),
	}
	if allocation.Properties != nil {
		namespace, ok := allocation.Properties.NamespaceLabels["kubernetes_io_metadata_name"]
		if ok {
			result.Namespace = lo.ToPtr(namespace)
		}
		if allocation.Properties.Container != "" {
			result.Container = lo.ToPtr(allocation.Properties.Container)
		}
	}
	namespace := ""
	if result.Namespace != nil {
		namespace = *result.Namespace
	}

	container, serviceID, err := r.getObjectInfo(ctx, resourceTypeEnum, namespace, name)
	if err != nil {
		return result
	}
	result.Container = container
	result.ServiceID = serviceID

	return result
}

func convertCostAttributes(allocation opencost.Allocation) *console.CostAttributes {
	attr := &console.CostAttributes{
		Memory:           lo.ToPtr(allocation.RAMBytes()),
		CPU:              lo.ToPtr(allocation.CPUCores()),
		Storage:          lo.ToPtr(allocation.PVBytes()),
		MemoryUtil:       lo.ToPtr(allocation.RAMBytesUsageAverage),
		CPUUtil:          lo.ToPtr(allocation.CPUCoreUsageAverage),
		CPUCost:          lo.ToPtr(allocation.CPUCost),
		MemoryCost:       lo.ToPtr(allocation.RAMCost),
		GpuCost:          lo.ToPtr(allocation.GPUCost),
		LoadBalancerCost: lo.ToPtr(allocation.LoadBalancerCost),
	}
	if allocation.GPUAllocation != nil {
		attr.GpuUtil = allocation.GPUAllocation.GPUUsageAverage
	}
	return attr
}
