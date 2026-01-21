package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CapiClusterController reconciler a v1alpha1.VirtualCluster resource.
// Implements [reconcile.Reconciler] interface.
type CapiClusterController struct {
	k8sClient.Client

	Scheme           *runtime.Scheme
	ExtConsoleClient client.Client
	ConsoleUrl       string

	userGroupCache cache.UserGroupCache
	consoleClient  client.Client
}

func (r *CapiClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := &clusterv1.Cluster{}

	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		logger.Info("Unable to fetch CAPI Cluster")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}
