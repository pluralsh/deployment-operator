package deploymentrequestpackage

import (
	"context"
	"fmt"

	crhelperTypes "github.com/pluralsh/controller-reconcile-helper/pkg/types"

	"github.com/go-logr/logr"
	platform "github.com/pluralsh/deployment-api/apis/platform/v1alpha1"
	"github.com/pluralsh/deployment-operator/pkg/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	DeploymentRequestFinalizer = "pluralsh.deployment-operator/deploymentrequest-protection"
)

// Reconciler reconciles a DeploymentRequest object
type Reconciler struct {
	client.Client
	Log logr.Logger
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("DeploymentRequest", req.NamespacedName)

	var deploymentRequest platform.DeploymentRequest
	if err := r.Get(ctx, req.NamespacedName, &deploymentRequest); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !deploymentRequest.DeletionTimestamp.IsZero() {
		deploymentRequestCopy := deploymentRequest.DeepCopy()
		if controllerutil.ContainsFinalizer(deploymentRequestCopy, DeploymentRequestFinalizer) {
			deployment := &platform.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: deploymentRequest.Spec.ExistingDeploymentName},
			}
			if err := r.Delete(ctx, deployment); err != nil {
				log.Error(err, "Error deleting deployment", "Deployment", deploymentRequest.Spec.ExistingDeploymentName)
				return ctrl.Result{}, err
			}
			log.Info("Successfully deleted deployment", "Deployment", deploymentRequest.Spec.ExistingDeploymentName)

			return ctrl.Result{}, kubernetes.TryRemoveFinalizer(ctx, r.Client, deploymentRequestCopy, DeploymentRequestFinalizer)
		}
		return ctrl.Result{}, nil
	}

	if !deploymentRequest.Status.Ready {
		if deploymentRequest.Spec.ExistingDeploymentName == "" {
			deploymentClassName := deploymentRequest.Spec.DeploymentClassName
			if deploymentClassName == "" {
				return ctrl.Result{}, fmt.Errorf("cannot find deployment class with the name specified in the deployment request")
			}

			var deploymentClass platform.DeploymentClass
			if err := r.Get(ctx, client.ObjectKey{Name: deploymentClassName}, &deploymentClass); err != nil {
				log.Error(err, "Can't get deployment class", "deploymentClass", deploymentClassName)
				return ctrl.Result{}, err
			}

			newDeployment := genDeployment(deploymentRequest, deploymentClass)
			if err := r.Get(ctx, client.ObjectKey{Name: newDeployment.Name}, &platform.Deployment{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return ctrl.Result{}, nil
				}
				if err := r.Create(ctx, newDeployment); err != nil {
					log.Error(err, "Can't create deployment")
					return ctrl.Result{}, err
				}
				log.Info("Successfully created deployment", "Deployment", newDeployment.Name)
			}
			deploymentRequest.Spec.ExistingDeploymentName = newDeployment.Name
			if err := r.Update(ctx, &deploymentRequest); err != nil {
				return ctrl.Result{}, err
			}
			if err := kubernetes.TryAddFinalizer(ctx, r.Client, &deploymentRequest, DeploymentRequestFinalizer); err != nil {
				return ctrl.Result{}, err
			}
			deploymentRequest.Status.Ready = false
			if err := r.Status().Update(ctx, &deploymentRequest); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func genDeployment(request platform.DeploymentRequest, class platform.DeploymentClass) *platform.Deployment {
	name := fmt.Sprintf("%s-%s", class.Name, request.Name)
	return &platform.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: platform.DeploymentSpec{
			DriverName:          class.DriverName,
			DeploymentClassName: class.Name,
			Parameters:          class.Parameters,
			DeploymentRequest: &corev1.ObjectReference{
				Name:      request.Name,
				Namespace: request.Namespace,
			},
			DeletionPolicy: class.DeletionPolicy,
		},
		Status: platform.DeploymentStatus{
			Ready:        false,
			DeploymentID: "",
			Conditions:   []crhelperTypes.Condition{},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platform.DeploymentRequest{}).
		Complete(r)
}
