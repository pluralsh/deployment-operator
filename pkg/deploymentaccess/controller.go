package deploymentaccess

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	platform "github.com/pluralsh/deployment-api/apis/platform/v1alpha1"
	deploymentspec "github.com/pluralsh/deployment-api/spec"
	deploymentctrl "github.com/pluralsh/deployment-operator/pkg/deployment"
	"github.com/pluralsh/deployment-operator/pkg/kubernetes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconciler reconciles a DeploymentAccess object
type Reconciler struct {
	client.Client
	Log logr.Logger

	DriverName        string
	ProvisionerClient deploymentspec.ProvisionerClient
}

const (
	SecretFinalizer           = "pluralsh.deployment-operator/secret-protection"
	DeploymentAccessFinalizer = "pluralsh.deployment-operator/deploymentaccess-protection"
)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("DeploymentAccess", req.NamespacedName)

	deploymentAccess := &platform.DeploymentAccess{}
	if err := r.Get(ctx, req.NamespacedName, deploymentAccess); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !deploymentAccess.GetDeletionTimestamp().IsZero() {
		if err := r.deleteDeploymentAccessOp(ctx, deploymentAccess); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if deploymentAccess.Status.AccessGranted && deploymentAccess.Status.AccountID != "" {
		log.Info("DeploymentAccess already exists")
		return ctrl.Result{}, nil
	}

	deploymentRequestName := deploymentAccess.Spec.DeploymentRequestName
	deploymentAccessClassName := deploymentAccess.Spec.DeploymentAccessClassName
	log.Info("Add DeploymentAccess")

	secretCredName := deploymentAccess.Spec.CredentialsSecretName
	if secretCredName == "" {
		return ctrl.Result{}, errors.New("CredentialsSecretName not defined in the DeploymentAccess")
	}

	deploymentAccessClass := &platform.DeploymentAccessClass{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentAccessClassName}, deploymentAccessClass); err != nil {
		log.Error(err, "Failed to get DeploymentAccessClass")
		return ctrl.Result{}, err
	}
	if !strings.EqualFold(deploymentAccessClass.DriverName, r.DriverName) {
		log.Info("Skipping DeploymentAccess for driver")
		return ctrl.Result{}, nil
	}

	namespace := deploymentAccess.ObjectMeta.Namespace
	deploymentRequest := &platform.DeploymentRequest{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentRequestName, Namespace: namespace}, deploymentRequest); err != nil {
		log.Error(err, "Failed to get DeploymentRequest")
		return ctrl.Result{}, err
	}
	if deploymentRequest.Status.DeploymentName == "" || deploymentRequest.Status.Ready != true {
		err := errors.New("DeploymentName cannot be empty or NotReady in deploymentRequest")
		return ctrl.Result{}, err
	}
	if deploymentAccess.Status.AccessGranted == true {
		log.Info("AccessAlreadyGranted")
		return ctrl.Result{}, nil
	}

	deployment := &platform.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentRequest.Status.DeploymentName}, deployment); err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}
	if deployment.Status.Ready != true || deployment.Status.DeploymentID == "" {
		err := errors.New("DeploymentAccess can't be granted to deployment not in Ready state and without a deploymentID")
		return ctrl.Result{}, err
	}

	accountName := fmt.Sprintf("%s-%s", "account", deploymentAccess.Name)
	grantAccessReq := &deploymentspec.DriverGrantDeploymentAccessRequest{
		Name:               accountName,
		AuthenticationType: 0,
		Parameters:         deploymentAccessClass.Parameters,
	}

	rsp, err := r.ProvisionerClient.DriverGrantDeploymentAccess(ctx, grantAccessReq)
	if err != nil {
		if status.Code(err) != codes.AlreadyExists {
			log.Error(err, "Failed to grant access")
			return ctrl.Result{}, err
		}

	}
	credentials := rsp.Credentials

	credentialSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretCredName, Namespace: namespace}, credentialSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "Failed to get credential secret")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:       secretCredName,
				Namespace:  namespace,
				Finalizers: []string{SecretFinalizer},
			},
			StringData: credentials["cred"].Secrets,
			Type:       corev1.SecretTypeOpaque,
		}); err != nil {
			log.Error(err, "Failed to create secret")
			return ctrl.Result{}, err
		}
	}

	if err := kubernetes.TryAddFinalizer(ctx, r.Client, deployment, deploymentctrl.DeploymentAccessFinalizer); err != nil {
		return ctrl.Result{}, err
	}
	if err := kubernetes.TryAddFinalizer(ctx, r.Client, deploymentAccess, DeploymentAccessFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	deploymentAccess.Status.AccountID = rsp.AccountId
	deploymentAccess.Status.AccessGranted = true

	if err := r.Status().Update(ctx, deploymentAccess); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) deleteDeploymentAccessOp(ctx context.Context, deploymentAccess *platform.DeploymentAccess) error {
	credSecretName := deploymentAccess.Spec.CredentialsSecretName
	credentialSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: credSecretName, Namespace: deploymentAccess.Namespace}, credentialSecret); err != nil {
		return err
	}
	if err := r.Delete(ctx, credentialSecret); err != nil {
		return err
	}

	if err := kubernetes.TryRemoveFinalizer(ctx, r.Client, credentialSecret, SecretFinalizer); err != nil {
		return err
	}

	if err := kubernetes.TryRemoveFinalizer(ctx, r.Client, deploymentAccess, DeploymentAccessFinalizer); err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platform.DeploymentAccess{}).
		Complete(r)
}
