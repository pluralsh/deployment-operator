package status

import (
	"context"
	"github.com/pluralsh/deployment-operator/pkg/kubernetes"
	"reflect"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/go-logr/logr"
	"github.com/pluralsh/controller-reconcile-helper/pkg/conditions"
	"github.com/pluralsh/controller-reconcile-helper/pkg/patch"
	crhelperTypes "github.com/pluralsh/controller-reconcile-helper/pkg/types"
	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler reconciles a Deployment object
type Reconciler struct {
	client.Client
	Log                 logr.Logger
	DeploymentNamespace string
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Application", req.NamespacedName)

	log.Info("fetch application")
	application := &v1alpha1.Application{}
	if err := r.Get(ctx, req.NamespacedName, application); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !application.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	log.Info("fetch deployment")
	deployment := &platform.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.DeploymentNamespace, Name: req.Name}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	patchHelper, err := patch.NewHelper(deployment, r.Client)
	if err != nil {
		log.Error(err, "Error getting patchHelper for Deployment")
		return ctrl.Result{}, err
	}
	if !deployment.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	newStatus := createStatus(application.Spec.Source.RepoURL, application.Status)
	currentStatus := deployment.Status.DeepCopy()
	currentStatus.Conditions = crhelperTypes.Conditions{}

	if reflect.DeepEqual(newStatus, currentStatus) {
		return ctrl.Result{}, nil
	}
	if err := kubernetes.UpdateDeploymentStatus(ctx, r.Client, deployment, func(d *platform.Deployment) {
		d.Status = newStatus
	}); err != nil {
		return ctrl.Result{}, err
	}
	if newStatus.Ready != currentStatus.Ready {
		conditions.MarkTrue(deployment, platform.DeploymentReadyCondition)
		if err := patchDeployment(ctx, patchHelper, deployment); err != nil {
			if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
				return reconcile.Result{RequeueAfter: time.Second * 1}, nil
			}
			log.Error(err, "failed to patch Deployment")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Application{}).
		Complete(r)
}

func createStatus(ref string, status v1alpha1.ApplicationStatus) platform.DeploymentStatus {
	ds := platform.DeploymentStatus{
		Ref:   ref,
		Ready: status.OperationState.Phase == synccommon.OperationSucceeded,
	}
	for _, resource := range status.Resources {
		ds.Resources = append(ds.Resources, platform.DeploymentResource{
			APIVersion: resource.Version,
			Kind:       resource.Kind,
			Name:       resource.Name,
			Namespace:  resource.Namespace,
			Status:     string(resource.Status),
			Synced:     resource.Status == v1alpha1.SyncStatusCodeUnknown,
		})
	}
	ds.Conditions = crhelperTypes.Conditions{}

	return ds
}

func patchDeployment(ctx context.Context, patchHelper *patch.Helper, deployment *platform.Deployment) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process (instead we are hiding it during the deletion process).
	conditions.SetSummary(deployment,
		conditions.WithConditions(
			platform.DeploymentReadyCondition,
		),
		conditions.WithStepCounterIf(deployment.ObjectMeta.DeletionTimestamp.IsZero()),
		conditions.WithStepCounter(),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		deployment,
		patch.WithOwnedConditions{Conditions: []crhelperTypes.ConditionType{
			crhelperTypes.ReadyCondition,
			platform.DeploymentReadyCondition,
		},
		},
	)
}
