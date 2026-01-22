package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ClusterNameLabel      = "pluralsh.com/cluster-name"
	ClusterNamespaceLabel = "pluralsh.com/cluster-namespace"
)

type CapiConfigurationClusterController struct {
	k8sClient.Client
	Scheme *runtime.Scheme
}

func (in *CapiConfigurationClusterController) Reconcile(ctx context.Context, req ctrl.Request) (_ reconcile.Result, reterr error) {
	logger := log.FromContext(ctx)

	configuration := &v1alpha1.CapiConfigurationCluster{}

	if err := in.Get(ctx, req.NamespacedName, configuration); err != nil {
		logger.Info("Unable to fetch CapiConfigurationCluster")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	utils.MarkCondition(configuration.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "")

	scope, err := NewDefaultScope(ctx, in.Client, configuration)
	if err != nil {
		logger.Error(err, "failed to create scope")
		utils.MarkCondition(configuration.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}

	// Always patch object when exiting this function, so we can persist any object changes.
	defer func() {
		if err := scope.PatchObject(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Synchronize the console token to make sure it is available
	_, err = configuration.GetConsoleToken(ctx, in.Client)
	if err != nil {
		if errors.IsNotFound(err) {
			utils.MarkCondition(configuration.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, "waiting for console token secret")
			return jitterRequeue(requeueAfter, jitter), nil
		}
		logger.Error(err, "failed to get console token from secret")
		utils.MarkCondition(configuration.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionFalse, v1alpha1.ReadyConditionReason, err.Error())
		return ctrl.Result{}, err
	}
	if configuration.Labels == nil {
		configuration.Labels = make(map[string]string)
	}
	configuration.Labels[ClusterNameLabel] = configuration.Spec.CapiCluster.Name
	configuration.Labels[ClusterNamespaceLabel] = configuration.Spec.CapiCluster.Namespace

	// Mark synchronized condition as true
	// The CAPI Cluster controller will handle the rest
	utils.MarkCondition(configuration.SetCondition, v1alpha1.ReadyConditionType, metav1.ConditionTrue, v1alpha1.ReadyConditionReason, "")

	return ctrl.Result{}, nil
}

func (in *CapiConfigurationClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(&v1alpha1.CapiConfigurationCluster{}).
		Complete(in)
}
