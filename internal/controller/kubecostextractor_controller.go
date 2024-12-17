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
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/opencost/opencost/core/pkg/opencost"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

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
func (r *KubecostExtractorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	kubecost := &v1alpha1.KubecostExtractor{}
	if err := r.Get(ctx, req.NamespacedName, kubecost); err != nil {
		logger.Error(err, "Unable to fetch kubecost")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ReadyConditionReason, "")

	if !kubecost.DeletionTimestamp.IsZero() {
		if cancel, exists := r.Tasks.Get(req.NamespacedName.String()); exists {
			cancel()
			r.Tasks.Remove(req.NamespacedName.String())
		}
	}

	// check service
	kubecostService := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: kubecost.Spec.KubecostServiceRef.Name, Namespace: kubecost.Spec.KubecostServiceRef.Namespace}, kubecostService); err != nil {
		logger.Error(err, "Unable to fetch service for kubecost")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	r.RunOnInterval(ctx, req.NamespacedName.String(), kubecost.Spec.GetInterval(), func(ctx context.Context) (done bool, err error) {
		clusterCostAttr, err := r.getClusterCost(ctx, kubecostService, kubecost.Spec.GetPort(), kubecost.Spec.GetInterval())
		if err != nil {
			logger.Error(err, "Unable to fetch cluster cost")
			utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
			return false, nil
		}

		if _, err := r.ExtConsoleClient.IngestClusterCost(console.CostIngestAttributes{
			Cluster:         clusterCostAttr,
			Namespaces:      nil,
			Recommendations: nil,
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

	// calculate window from interval
	now := time.Now()
	subtractedTime := now.Add(-interval)
	window := fmt.Sprintf("%d,%d", subtractedTime.Unix(), now.Unix())

	queryParams := map[string]string{
		"window":    window,
		"aggregate": "cluster",
	}

	bytes, err = r.KubeClient.CoreV1().Services(srv.Namespace).ProxyGet("", srv.Name, servicePort, "/model/allocation", queryParams).DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	var ar allocationResponse
	if err = json.Unmarshal(bytes, &ar); err != nil {
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

	return nil, fmt.Errorf("no cluster cost found for service: %s", srv.Name)
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

func convertCostAttributes(allocation opencost.Allocation) *console.CostAttributes {
	attr := &console.CostAttributes{
		Memory:           lo.ToPtr(allocation.RAMBytes()),
		CPU:              lo.ToPtr(allocation.CPUCores()),
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
