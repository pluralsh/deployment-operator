package controller

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts"
	rolloutv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	roclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	inventoryAnnotationName = "config.k8s.io/owning-inventory"
	closed                  = "closed"
)

var requeueRollout = ctrl.Result{RequeueAfter: time.Second * 5}

// ArgoRolloutReconciler reconciles a Argo Rollout custom resource.
type ArgoRolloutReconciler struct {
	k8sClient.Client
	Scheme        *runtime.Scheme
	ConsoleClient client.Client
	ConsoleURL    string
	CachedClient  *utils.CachedClient
	ArgoClientSet roclientset.Interface
}

// Reconcile Argo Rollout custom resources to ensure that Console stays in sync with Kubernetes cluster.
func (r *ArgoRolloutReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Read resource from Kubernetes cluster.
	rollout := &rolloutv1alpha1.Rollout{}
	if err := r.Get(ctx, req.NamespacedName, rollout); err != nil {
		logger.Error(err, "unable to fetch rollout")
		return ctrl.Result{}, k8sClient.IgnoreNotFound(err)
	}

	if !rollout.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	serviceID, ok := rollout.Annotations[inventoryAnnotationName]
	if !ok {
		return ctrl.Result{}, nil
	}
	if serviceID == "" {
		return ctrl.Result{}, fmt.Errorf("the service ID from the inventory annotation is empty")
	}
	service, err := r.ConsoleClient.GetService(serviceID)
	if err != nil {
		return ctrl.Result{}, err
	}
	consoleURL, err := sanitizeURL(r.ConsoleURL)
	if err != nil {
		return ctrl.Result{}, err
	}
	if hasPausedRolloutComponent(service) {
		promoteURL := fmt.Sprintf("%s/ext/v1/gate/%s", consoleURL, serviceID)
		rollbackURL := fmt.Sprintf("%s/ext/v1/rollback/%s", consoleURL, serviceID)
		promoteResponse, err := r.CachedClient.Get(promoteURL)
		if err != nil {
			return ctrl.Result{}, err
		}
		rollbackResponse, err := r.CachedClient.Get(rollbackURL)
		if err != nil {
			return ctrl.Result{}, err
		}

		if promoteResponse != closed {
			rolloutIf := r.ArgoClientSet.ArgoprojV1alpha1().Rollouts(rollout.Namespace)
			if _, err := utils.PromoteRollout(ctx, rolloutIf, rollout.Name); err != nil {
				return ctrl.Result{}, err
			}
		}
		if rollbackResponse != closed {

		}
		return requeueRollout, nil
	}
	return ctrl.Result{}, nil
}

func hasPausedRolloutComponent(service *console.GetServiceDeploymentForAgent_ServiceDeployment) bool {
	for _, component := range service.Components {
		if component.Kind == rollouts.RolloutKind {
			if component.State != nil && *component.State == console.ComponentStatePaused {
				return true
			}
		}
	}
	return false
}

func sanitizeURL(consoleURL string) (string, error) {
	u, err := url.Parse(consoleURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ArgoRolloutReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rolloutv1alpha1.Rollout{}).
		Complete(r)
}