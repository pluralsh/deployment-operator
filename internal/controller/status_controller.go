package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

type StatusReconciler struct {
	k8sClient.Client
	inventoryCache cache.InventoryResourceKeys
}

func (r *StatusReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, req.NamespacedName, configMap); err != nil {
		logger.Info("unable to fetch configmap")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	if !configMap.DeletionTimestamp.IsZero() {
		return r.handleDelete(configMap)
	}

	inv, err := toUnstructured(configMap)
	if err != nil {
		return ctrl.Result{}, err
	}

	set, err := inventory.WrapInventoryObj(inv).Load()
	if err != nil {
		return ctrl.Result{}, err
	}

	invID := r.inventoryID(configMap)

	// If services arg is provided, we can skip
	// services that are not on the list.
	if args.SkipService(invID) {
		return ctrl.Result{}, nil
	}

	r.inventoryCache[invID] = cache.ResourceKeyFromObjMetadata(set)
	cache.GetResourceCache().Register(r.inventoryCache.Values().TypeIdentifierSet())

	return ctrl.Result{}, nil
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

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: objMap}, nil
}

func (r *StatusReconciler) inventoryID(c *corev1.ConfigMap) string {
	return c.Labels[common.InventoryLabel]
}

func (r *StatusReconciler) handleDelete(c *corev1.ConfigMap) (ctrl.Result, error) {
	inventoryID := r.inventoryID(c)
	delete(r.inventoryCache, inventoryID)

	return ctrl.Result{}, nil
}

func NewStatusReconciler(c client.Client) (*StatusReconciler, error) {
	return &StatusReconciler{
		Client:         c,
		inventoryCache: make(cache.InventoryResourceKeys),
	}, nil
}
