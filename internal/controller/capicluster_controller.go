package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CapiClusterController reconciler a v1alpha1.VirtualCluster resource.
// Implements [reconcile.Reconciler] interface.
type CapiClusterController struct {
	k8sClient.Client
	Scheme        *runtime.Scheme
	consoleClient client.Client
}

func (r *CapiClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := &clusterv1.Cluster{}

	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		logger.Info("Unable to fetch CAPI Cluster")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	if cluster.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	if !r.isClusterReady(cluster) {
		logger.Info("CAPI Cluster is not ready yet", "cluster", cluster.Name)
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *CapiClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&clusterv1.Cluster{}).
		Complete(r)
}

func (r *CapiClusterController) isClusterReady(cluster *clusterv1.Cluster) bool {
	// Check if cluster phase is Provisioned
	if cluster.Status.Phase != string(clusterv1.ClusterPhaseProvisioned) {
		return false
	}

	// Check Available condition - this is the main readiness indicator in v1beta2
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == clusterv1.AvailableCondition {
			return condition.Status == "True"
		}
	}

	return false
}
