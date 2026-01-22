package controller

import (
	"context"
	"fmt"

	"github.com/docker/docker/daemon/logger"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	utilkubeconfig "sigs.k8s.io/cluster-api/util/kubeconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CapiClusterController reconciler a v1alpha1.VirtualCluster resource.
// Implements [reconcile.Reconciler] interface.
type CapiClusterController struct {
	k8sClient.Client
	Scheme     *runtime.Scheme
	ConsoleUrl string

	consoleClient client.Client
}

func (in *CapiClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cluster := &clusterv1.Cluster{}
	if err := in.Get(ctx, req.NamespacedName, cluster); err != nil {
		logger.Info("Unable to fetch CAPI Cluster")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	if cluster.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	clusterConfiguration := in.getClusterConfiguration(ctx, cluster)
	// Wait until the cluster configuration is ready
	if clusterConfiguration == nil {
		return jitterRequeue(requeueAfter, jitter), nil
	}

	if err := in.initConsoleClient(ctx, clusterConfiguration); err != nil {
		logger.Error(err, "Unable to initialize console client")
		return ctrl.Result{}, err
	}

	consoleClusterID, deployToken, err := in.syncConsoleCluster(clusterConfiguration)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (in *CapiClusterController) syncConsoleCluster(configuration *v1alpha1.CapiConfigurationCluster) (id, token string, err error) {
	existingConsoleCluster, err := in.consoleClient.GetClusterByHandle(configuration.ClusterName())
	if err != nil {
		if errors.IsNotFound(err) {
			newConsoleCluster, err := in.consoleClient.CreateCluster(console.ClusterAttributes{
				Name:   configuration.ClusterName(),
				Handle: lo.ToPtr(configuration.ClusterName()),
			})
			if err != nil {
				return
			}
			if newConsoleCluster.CreateCluster.DeployToken == nil {
				return "", "", fmt.Errorf("could not fetch deploy token from cluster")
			}
			return newConsoleCluster.CreateCluster.ID, lo.FromPtr(newConsoleCluster.CreateCluster.DeployToken), nil
		}
		return
	}
	id = existingConsoleCluster.ID
	token, err = in.consoleClient.GetDeployToken(&id, nil)
	return
}

func (in *CapiClusterController) getKubeconfig(ctx context.Context, cluster *clusterv1.Cluster) ([]byte, error) {
	obj := k8sClient.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	}
	return utilkubeconfig.FromSecret(ctx, in.Client, obj)
}

func (in *CapiClusterController) initConsoleClient(ctx context.Context, configuration *v1alpha1.CapiConfigurationCluster) error {
	if in.consoleClient == nil {
		token, err := configuration.GetConsoleToken(ctx, in.Client)
		if err != nil {
			return err
		}
		in.consoleClient = client.New(in.ConsoleUrl, token)
	}
	return nil
}

func (in *CapiClusterController) getClusterConfiguration(ctx context.Context, cluster *clusterv1.Cluster) *v1alpha1.CapiConfigurationCluster {
	configurationList := &v1alpha1.CapiConfigurationClusterList{}

	// Create a label selector to match configurations for this cluster
	labelSelector := k8sClient.MatchingLabels{
		ClusterNameLabel:      cluster.Name,
		ClusterNamespaceLabel: cluster.Namespace,
	}

	// List with label selector
	if err := in.List(ctx, configurationList, labelSelector); err != nil {
		return nil
	}
	for _, config := range configurationList.Items {
		if meta.IsStatusConditionTrue(config.Status.Conditions, string(v1alpha1.ReadyConditionType)) {
			return config.DeepCopy()
		}
	}

	return nil
}

func (in *CapiClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&clusterv1.Cluster{}, builder.WithPredicates(clusterStatusPredicate())).
		Complete(in)
}

func clusterStatusPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			cluster, ok := e.Object.(*clusterv1.Cluster)
			if !ok {
				return false
			}
			// Only trigger if the cluster is ready
			// to avoid unnecessary reconciliations
			return isClusterReady(cluster)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			newCluster, okNew := e.ObjectNew.(*clusterv1.Cluster)
			if !okNew {
				return false
			}
			return isClusterReady(newCluster)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func isClusterReady(cluster *clusterv1.Cluster) bool {
	// Check if cluster phase is Provisioned
	if cluster.Status.Phase != string(clusterv1.ClusterPhaseProvisioned) {
		return false
	}

	return meta.IsStatusConditionTrue(cluster.Status.Conditions, clusterv1.AvailableCondition)
}
