package controller

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ConstraintReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ConsoleClient consoleclient.Client
	Reader        client.Reader
}

func (r *ConstraintReconciler) Reconcile(ctx context.Context, req ctrl.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	cps := new(constraintstatusv1beta1.ConstraintPodStatus)
	if err := r.Get(ctx, req.NamespacedName, cps); err != nil {
		logger.Error(err, "Unable to fetch ConstraintPodStatus")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}
	if !cps.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	instance, err := ConstraintPodStatusToUnstructured(ctx, cps)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Reader.Get(ctx, types.NamespacedName{Name: instance.GetName()}, instance); err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *ConstraintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&constraintstatusv1beta1.ConstraintPodStatus{}).
		Complete(r)
}

func ConstraintPodStatusToUnstructured(ctx context.Context, cps *constraintstatusv1beta1.ConstraintPodStatus) (*unstructured.Unstructured, error) {
	logger := log.FromContext(ctx)
	labels := cps.GetLabels()
	name, ok := labels[v1beta1.ConstraintNameLabel]
	if !ok {
		err := fmt.Errorf("constraint status resource with no name label: %s", cps.GetName())
		logger.Error(err, "missing label while attempting to map a constraint status resource")
		return nil, err
	}
	kind, ok := labels[v1beta1.ConstraintKindLabel]
	if !ok {
		err := fmt.Errorf("constraint status resource with no kind label: %s", cps.GetName())
		logger.Error(err, "missing label while attempting to map a constraint status resource")
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: v1beta1.ConstraintsGroup, Version: "v1beta1", Kind: kind})
	u.SetName(name)
	return u, nil
}
