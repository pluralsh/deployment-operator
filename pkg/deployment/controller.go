package deployment

import (
	"context"
	"errors"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"github.com/pluralsh/controller-reconcile-helper/pkg/conditions"
	"github.com/pluralsh/controller-reconcile-helper/pkg/patch"
	crhelperTypes "github.com/pluralsh/controller-reconcile-helper/pkg/types"
	platform "github.com/pluralsh/deployment-api/apis/platform/v1alpha1"
	deploymentspec "github.com/pluralsh/deployment-api/spec"
	"github.com/pluralsh/deployment-operator/pkg/kubernetes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DeploymentAccessFinalizer  = "pluralsh.deployment-operator/deploymentaccess-deployment-protection"
	DeploymentFinalizer        = "pluralsh.deployment-operator/deployment-protection"
	DeploymentRequestFinalizer = "pluralsh.deployment-operator/deploymentrequest-protection"
)

// Reconciler reconciles a DeploymentRequest object
type Reconciler struct {
	client.Client
	Log logr.Logger

	DriverName        string
	ProvisionerClient deploymentspec.ProvisionerClient
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Deployment", req.NamespacedName)

	deployment := &platform.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, deployment); err != nil {
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
		if controllerutil.ContainsFinalizer(deployment, DeploymentAccessFinalizer) {
			deploymentReqNs := deployment.Spec.DeploymentRequest.Namespace
			deploymentReqName := deployment.Spec.DeploymentRequest.Name

			var deploymentAccessList platform.DeploymentAccessList
			if err := r.List(ctx, &deploymentAccessList, client.InNamespace(deploymentReqNs)); err != nil {
				log.Error(err, "Failed to get DeploymentAccessList")
				return ctrl.Result{}, err
			}
			for _, deploymentAccess := range deploymentAccessList.Items {
				if strings.EqualFold(deploymentAccess.Spec.DeploymentRequestName, deploymentReqName) {
					if err := r.Delete(ctx, &platform.DeploymentAccess{
						ObjectMeta: metav1.ObjectMeta{Name: deploymentAccess.Name, Namespace: deploymentReqNs},
					}); err != nil {
						log.Error(err, "Failed to delete DeploymentAccess")
						return ctrl.Result{}, err
					}
				}
			}
			if err := kubernetes.TryRemoveFinalizer(ctx, r.Client, deployment, DeploymentAccessFinalizer); err != nil {
				return ctrl.Result{}, err
			}
		}
		if controllerutil.ContainsFinalizer(deployment, DeploymentFinalizer) {
			if err := r.deleteDeploymentOp(ctx, deployment); err != nil {
				log.Error(err, "Failed to delete Deployment")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !strings.EqualFold(deployment.Spec.DriverName, r.DriverName) {
		return ctrl.Result{}, nil
	}

	if deployment.Status.Ready {
		return ctrl.Result{}, nil
	}

	deploymentReady := false
	var deploymentID string

	if deployment.Spec.ExistingDeploymentID == "" {
		req := &deploymentspec.DriverCreateDeploymentRequest{
			Parameters: deployment.Spec.Parameters,
			Name:       deployment.ObjectMeta.Name,
		}
		rsp, err := r.ProvisionerClient.DriverCreateDeployment(ctx, req)
		if err != nil {
			if status.Code(err) != codes.AlreadyExists {
				conditions.MarkFalse(deployment, platform.DeploymentReadyCondition, platform.FailedToCreateDeploymentReason, crhelperTypes.ConditionSeverityError, err.Error())
				if err := patchDeployment(ctx, patchHelper, deployment); err != nil {
					log.Error(err, "failed to patch Deployment")
					return ctrl.Result{}, err
				}
				log.Error(err, "Driver failed to create deployment")
				return ctrl.Result{}, err
			}
		}
		if rsp == nil {
			err = errors.New("DriverCreateDeployment returned a nil response")
			log.Error(err, "Internal Error from driver")
			return ctrl.Result{}, err
		}

		if rsp.DeploymentId != "" {
			deploymentID = rsp.DeploymentId
			deploymentReady = true
		} else {
			log.Error(err, "DriverCreateDeployment returned an empty deploymentID")
			err = errors.New("DriverCreateDeployment returned an empty deploymentID")
			return ctrl.Result{}, err
		}
		conditions.MarkTrue(deployment, platform.DeploymentReadyCondition)

		// Now we update the DeploymentReady status of DeploymentRequest
		if deployment.Spec.DeploymentRequest != nil {
			ref := deployment.Spec.DeploymentRequest

			deploymentReq := &platform.DeploymentRequest{}
			if err := r.Get(ctx, client.ObjectKey{
				Namespace: ref.Namespace,
				Name:      ref.Name,
			}, deploymentReq); err != nil {
				log.Error(err, "Failed to get deployment request")
				return ctrl.Result{}, err
			}

			deploymentReq.Status.Ready = true
			deploymentReq.Status.DeploymentName = deployment.Name
			if err := r.Status().Update(ctx, deploymentReq); err != nil {
				if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
					return reconcile.Result{RequeueAfter: time.Second * 1}, nil
				}
				log.Error(err, "Failed to update DeploymentRequest status")
				return ctrl.Result{}, err
			}
			log.Info("Successfully updated status of DeploymentRequest")
		}
	} else {
		deploymentReady = true
		deploymentID = deployment.Spec.ExistingDeploymentID
	}

	deployment.Status.Ready = deploymentReady
	deployment.Status.DeploymentID = deploymentID
	if err := r.Status().Update(ctx, deployment); err != nil {
		log.Error(err, "Can't update deployment")
		return ctrl.Result{}, err
	}

	if err := kubernetes.TryAddFinalizer(ctx, r.Client, deployment, DeploymentFinalizer); err != nil {
		log.Error(err, "Can't update finalizer")
		return ctrl.Result{}, err
	}

	if err := patchDeployment(ctx, patchHelper, deployment); err != nil {
		if strings.Contains(err.Error(), genericregistry.OptimisticLockErrorMsg) {
			return reconcile.Result{RequeueAfter: time.Second * 1}, nil
		}
		log.Error(err, "failed to patch Deployment")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) deleteDeploymentOp(ctx context.Context, deployment *platform.Deployment) error {
	if !strings.EqualFold(deployment.Spec.DriverName, r.DriverName) {
		return nil
	}

	if deployment.Spec.DeletionPolicy == platform.DeletionPolicyDelete {
		req := &deploymentspec.DriverDeleteDeploymentRequest{
			DeploymentId: deployment.Status.DeploymentID,
		}
		if _, err := r.ProvisionerClient.DriverDeleteDeployment(ctx, req); err != nil {
			if status.Code(err) != codes.NotFound {
				return err
			}
		}
	}

	kubernetes.TryRemoveFinalizer(ctx, r.Client, deployment, DeploymentFinalizer)

	if deployment.Spec.DeploymentRequest != nil {
		ref := deployment.Spec.DeploymentRequest
		deploymentRequest := &platform.DeploymentRequest{}
		if err := r.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, deploymentRequest); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
		return kubernetes.TryRemoveFinalizer(ctx, r.Client, deploymentRequest, DeploymentRequestFinalizer)
	}

	return nil
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

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platform.Deployment{}).
		Complete(r)
}
