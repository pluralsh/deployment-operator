package deployment

import (
	"context"
	"errors"
	"github.com/go-logr/logr"
	"github.com/pluralsh/controller-reconcile-helper/pkg/conditions"
	"github.com/pluralsh/controller-reconcile-helper/pkg/patch"
	crhelperTypes "github.com/pluralsh/controller-reconcile-helper/pkg/types"
	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/kubernetes"
	proto "github.com/pluralsh/deployment-operator/provisioner/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
)

const (
	DeploymentFinalizer = "pluralsh.deployment-operator/deployment-protection"
)

// Reconciler reconciles a Deployment object
type Reconciler struct {
	client.Client
	Log logr.Logger

	ProviderName      string
	ProvisionerClient proto.ProvisionerClient
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
		if controllerutil.ContainsFinalizer(deployment, DeploymentFinalizer) {
			if err := r.deleteDeploymentOp(ctx, deployment); err != nil {
				log.Error(err, "Failed to delete Deployment")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !strings.EqualFold(deployment.Spec.ProviderName, r.ProviderName) {
		return ctrl.Result{}, nil
	}

	if deployment.Spec.ExistingDeploymentID == "" {
		req := &proto.ProviderCreateDeploymentRequest{
			Name:      deployment.ObjectMeta.Name,
			Namespace: deployment.Spec.Namespace,
			Git: &proto.GitRef{
				Ref:    deployment.Spec.Git.Ref,
				Folder: deployment.Spec.Git.Folder,
			},
			Revision: &proto.Revision{
				Version: deployment.Spec.Revision.Version,
			},
			Parameters: deployment.Spec.Parameters,
		}
		rsp, err := r.ProvisionerClient.ProviderCreateDeployment(ctx, req)
		if err != nil {
			if status.Code(err) != codes.AlreadyExists {
				conditions.MarkFalse(deployment, platform.DeploymentReadyCondition, platform.FailedToCreateDeploymentReason, crhelperTypes.ConditionSeverityError, err.Error())
				if err := patchDeployment(ctx, patchHelper, deployment); err != nil {
					log.Error(err, "failed to patch Deployment")
					return ctrl.Result{}, err
				}
				log.Error(err, "Provider failed to create deployment")
				return ctrl.Result{}, err
			}
		}
		if rsp == nil {
			err = errors.New("ProviderCreateDeployment returned a nil response")
			log.Error(err, "Internal Error from provider")
			return ctrl.Result{}, err
		}

		if rsp.DeploymentId != "" {
			if err := kubernetes.UpdateDeployment(ctx, r.Client, deployment, func(d *platform.Deployment) {
				d.Spec.ExistingDeploymentID = rsp.DeploymentId
			}); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "ProviderCreateDeployment returned an empty deploymentID")
			err = errors.New("ProviderCreateDeployment returned an empty deploymentID")
			return ctrl.Result{}, err
		}

		if err := kubernetes.TryAddFinalizer(ctx, r.Client, deployment, DeploymentFinalizer); err != nil {
			log.Error(err, "Can't update finalizer")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) deleteDeploymentOp(ctx context.Context, deployment *platform.Deployment) error {
	if !strings.EqualFold(deployment.Spec.ProviderName, r.ProviderName) {
		return nil
	}

	if deployment.Spec.DeletionPolicy == platform.DeletionPolicyDelete {
		req := &proto.ProviderDeleteDeploymentRequest{
			DeploymentId: deployment.Status.DeploymentID,
		}
		if _, err := r.ProvisionerClient.ProviderDeleteDeployment(ctx, req); err != nil {
			if status.Code(err) != codes.NotFound {
				return err
			}
		}
	}

	return kubernetes.TryRemoveFinalizer(ctx, r.Client, deployment, DeploymentFinalizer)
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
