package controller

import (
	"context"
	"slices"

	"sigs.k8s.io/cli-utils/pkg/object"

	"sigs.k8s.io/cli-utils/pkg/apply/prune"

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

	pruner *prune.Pruner
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

	inv, err := toUnstructured(configmap)
	if err != nil {
		return ctrl.Result{}, err
	}

	set, err := r.pruner.GetPruneObjs(inventory.WrapInventoryInfoObj(inv), object.UnstructuredSet{}, prune.Options{
		DryRunStrategy: common.DryRunServer,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	invID := configmap.Labels[common.InventoryLabel]
	r.inventoryCache[invID] = getInventoryObj(set)

	values := slices.Concat(lo.Values(r.inventoryCache))

	resourceCache, err := cache.GetResourceCache()
	if err != nil {
		return ctrl.Result{}, err
	}
	resourceCache.Register(lo.Assign(values...))

	return ctrl.Result{}, nil
}

func getInventoryObj(l []*unstructured.Unstructured) map[string]*unstructured.Unstructured {
	result := map[string]*unstructured.Unstructured{}
	for _, u := range l {
		result[cache.ToResourceKey(u)] = u
	}

	return result
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
	f := utils.NewFactory(config)
	invFactory := inventory.ClusterClientFactory{StatusPolicy: inventory.StatusPolicyNone}
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return nil, err
	}
	prunner, err := prune.NewPruner(f, invClient)
	if err != nil {
		return nil, err
	}
	return &StatusReconciler{
		Client:         c,
		config:         config,
		inventoryCache: make(map[string]map[string]*unstructured.Unstructured),
		pruner:         prunner,
	}, nil
}
