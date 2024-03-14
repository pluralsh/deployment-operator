package controller

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/source"

	constraintstatusv1beta1 "github.com/open-policy-agent/gatekeeper/v3/apis/status/v1beta1"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/controller/constraintstatus"
	"github.com/open-policy-agent/gatekeeper/v3/pkg/util"
	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ConstraintReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	ConsoleClient consoleclient.Client
	Reader        client.Reader
}

func (r *ConstraintReconciler) Reconcile(ctx context.Context, request ctrl.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	gvk, unpackedRequest, err := util.UnpackRequest(request)
	if err != nil {
		// Unrecoverable, do not retry.
		logger.Error(err, "unpacking request", "request", request)
		return reconcile.Result{}, nil
	}
	// Sanity - make sure it is a constraint resource.
	if gvk.Group != constraintstatusv1beta1.ConstraintsGroup {
		// Unrecoverable, do not retry.
		logger.Error(err, "invalid constraint GroupVersion", "gvk", gvk)
		return reconcile.Result{}, nil
	}

	instance := &unstructured.Unstructured{}
	instance.SetGroupVersionKind(gvk)

	if err := r.Reader.Get(ctx, unpackedRequest.NamespacedName, instance); err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *ConstraintReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).Build(r)
	if err != nil {
		return err
	}
	if err := c.Watch(source.Kind(mgr.GetCache(), &constraintstatusv1beta1.ConstraintPodStatus{}), handler.EnqueueRequestsFromMapFunc(constraintstatus.PodStatusToConstraintMapper(true, util.EventPackerMapFunc()))); err != nil {
		return err
	}
	return nil
}
