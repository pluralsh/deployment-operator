package controller

import (
	"context"
	"slices"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

type StatusReconciler struct {
	k8sClient.Client

	// inventoryCache maps cli-utils inventory ID to a map of resourceKey - unstructured pairs.
	inventoryCache map[string]map[string]*unstructured.Unstructured
	config         *rest.Config
}

func (r *StatusReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	configmap := &corev1.ConfigMap{}
	if err := r.Get(ctx, req.NamespacedName, configmap); err != nil {
		logger.Error(err, "unable to fetch configmap")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	// TODO: handle delete and cleanup watches
	if !configmap.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	f := utils.NewFactory(r.config)
	invFactory := inventory.ClusterClientFactory{StatusPolicy: inventory.StatusPolicyNone}
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return ctrl.Result{}, err
	}
	inv, err := toUnstructured(configmap)
	if err != nil {
		return ctrl.Result{}, err
	}

	set, err := invClient.GetClusterInventoryObjs(inventory.WrapInventoryInfoObj(inv))
	if err != nil {
		return ctrl.Result{}, err
	}

	invID := configmap.Labels[common.InventoryLabel]

	resourceMap, ok := r.inventoryCache[invID]
	if !ok {
		resourceMap = map[string]*unstructured.Unstructured{}
	}

	for _, obj := range set {
		resourceMap[cache.ToResourceKey(obj)] = obj
	}
	values := slices.Concat(lo.Values(r.inventoryCache))

	serverCache, err := cache.GetServerCache()
	if err != nil {
		return ctrl.Result{}, err
	}
	serverCache.Register(lo.Assign(values...))

	return ctrl.Result{}, nil
}

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: objMap}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(o client.Object) bool {
			_, exists := o.GetLabels()[common.InventoryLabel]
			return exists
		})).
		Complete(r)
}

func NewStatusReconciler(c client.Client, config *rest.Config) (*StatusReconciler, error) {
	return &StatusReconciler{
		Client:         c,
		config:         config,
		inventoryCache: make(map[string]map[string]*unstructured.Unstructured),
	}, nil
}
