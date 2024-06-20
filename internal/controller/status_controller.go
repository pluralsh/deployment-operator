package controller

import (
	"context"

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
)

type StatusReconciler struct {
	k8sClient.Client

	config *rest.Config
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
	logger.Info("inventory objects", "set", set)

		//statusWatcher := watcher.NewDefaultStatusWatcher(dynamicClient, mapper)
		//statusWatcher.Filters = watcher.Filters{
		//	Labels: nil,
		//	Fields: nil,
		//}
		//ctx, cancelFunc := context.WithCancel(context.Background())
		//eventCh := statusWatcher.Watch(ctx, ids, watcher.Options{})
		//statusWatcher.Mapper
	//	for e := range eventCh {
	//	   // Handle event
	//	   if e.Type == event.ErrorEvent {
	//	     cancelFunc()
	//	     return e.Err
	//	   }
	//	}

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
		Client: c,
		config: config,
	}, nil
}
