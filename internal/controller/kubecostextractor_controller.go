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
	"fmt"

	console "github.com/pluralsh/console/go/client"
	corev1 "k8s.io/api/core/v1"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
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
	Scheme     *runtime.Scheme
	KubeClient kubernetes.Interface
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

	}

	kubecostService := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: kubecost.Spec.KubecostServiceRef.Name, Namespace: kubecost.Spec.KubecostServiceRef.Namespace}, kubecostService); err != nil {
		logger.Error(err, "Unable to fetch service for kubecost")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	_, err := r.getClusterCost(ctx, kubecostService, kubecost.Spec.GetPort(), "30d")
	if err != nil {
		logger.Error(err, "Unable to fetch cluster cost")
		utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionFalse, v1alpha1.ErrorConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	utils.MarkCondition(kubecost.SetCondition, v1alpha1.ReadyConditionType, v1.ConditionTrue, v1alpha1.ReadyConditionReason, "")
	return ctrl.Result{}, nil
}

func (r *KubecostExtractorReconciler) getClusterCost(ctx context.Context, srv *corev1.Service, servicePort, interval string) (*console.CostAttributes, error) {
	attr := &console.CostAttributes{
		Namespace:        nil,
		Memory:           nil,
		CPU:              nil,
		Gpu:              nil,
		Storage:          nil,
		MemoryUtil:       nil,
		CPUUtil:          nil,
		GpuUtil:          nil,
		CPUCost:          nil,
		MemoryCost:       nil,
		GpuCost:          nil,
		IngressCost:      nil,
		LoadBalancerCost: nil,
		EgressCost:       nil,
	}

	queryParams := map[string]string{
		"window":    interval,
		"aggregate": "cluster",
	}

	bytes, err := r.KubeClient.CoreV1().Services(srv.Namespace).ProxyGet("", srv.Name, servicePort, "/model/allocation", queryParams).DoRaw(ctx)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(bytes))

	return attr, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubecostExtractorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubecostExtractor{}).
		Complete(r)
}
