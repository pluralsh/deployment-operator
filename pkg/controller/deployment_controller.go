package controller

import (
	"context"

	"github.com/pluralsh/deployment-operator/apis/platform/v1alpha1"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Log        *zap.SugaredLogger
	Namespace  string
	Scheme     *runtime.Scheme
	Kubeconfig string
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Application{}).
		Complete(r)
}
