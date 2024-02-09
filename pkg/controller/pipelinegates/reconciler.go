package pipelinegates

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/pluralsh/deployment-operator/api/v1alpha1"
	pgctrl "github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/utils"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	Local = false
}

var (
	Local = false
)

type GateReconciler struct {
	K8sClient       ctrlclient.Client
	ConsoleClient   client.Client
	Config          *rest.Config
	Clientset       *kubernetes.Clientset
	GateCache       *client.Cache[console.PipelineGateFragment]
	GateQueue       workqueue.RateLimitingInterface
	UtilFactory     util.Factory
	discoveryClient *discovery.DiscoveryClient
	pinger          *ping.Pinger
}

func NewGateReconciler(consoleClient client.Client, k8sClient ctrlclient.Client, config *rest.Config, refresh time.Duration, clusterId string) (*GateReconciler, error) {
	utils.DisableClientLimits(config)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	gateCache := client.NewCache[console.PipelineGateFragment](refresh, func(id string) (*console.PipelineGateFragment, error) {
		return consoleClient.GetClusterGate(id)
	})

	gateQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	f := utils.NewFactory(config)

	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	return &GateReconciler{
		K8sClient:       k8sClient,
		ConsoleClient:   consoleClient,
		Config:          config,
		Clientset:       cs,
		GateQueue:       gateQueue,
		GateCache:       gateCache,
		UtilFactory:     f,
		discoveryClient: discoveryClient,
		pinger:          ping.New(consoleClient, discoveryClient, f),
	}, nil
}

func (s *GateReconciler) WipeCache() {
	s.GateCache.Wipe()
}

func (s *GateReconciler) ShutdownQueue() {
	s.GateQueue.ShutDown()
}

func (s *GateReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)

	logger.Info("fetching gates for cluster")
	gates, err := s.ConsoleClient.GetClusterGates()
	if err != nil {
		logger.Error(err, "failed to fetch gates list")
		return false, nil
	}

	for _, gate := range gates {
		logger.Info("sending update for", "gate", gate.ID)
		s.GateQueue.Add(gate.ID)
	}

	if err := s.pinger.Ping(); err != nil {
		logger.Error(err, "failed to ping cluster after scheduling syncs")
	}

	return false, nil
}

func (s *GateReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info("attempting to sync gate", "id", id)
	gate, err := s.GateCache.Get(id)
	if err != nil {
		logger.Error(err, "failed to fetch gate: %s, ignoring for now")
		return
	}

	logger.Info("attempting to sync gate", "Name", gate.Name, "ID", gate.ID)

	if gate.Type != console.GateTypeJob {
		logger.Info(fmt.Sprintf("gate is of type %s, we only reconcile gates of type %s skipping", gate.Type, console.GateTypeJob), "Name", gate.Name, "ID", gate.ID)
	}

	gateCR, err := s.ConsoleClient.ParsePipelineGateCR(gate)
	if err != nil {
		logger.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
		return reconcile.Result{}, err
	}

	preserveStatus := gateCR.Status.DeepCopy()

	nsName := types.NamespacedName{
		Name:      gateCR.Name,
		Namespace: gateCR.Namespace,
	}
	currentGate := &v1alpha1.PipelineGate{}

	// get pipelinegate
	if err := s.K8sClient.Get(ctx, nsName, currentGate); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("This gate doesn't yet have a corresponding CR on this cluster yet.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
			// If the PipelineGate doesn't exist, create it.
			err = s.K8sClient.Create(context.Background(), gateCR)
			if err != nil {
				logger.Error(err, "failed to create gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
				return reconcile.Result{}, err
			}
			gateCR.Status = *preserveStatus

			err = s.K8sClient.Status().Update(context.Background(), gateCR)
			if err != nil {
				logger.Error(err, "failed to create gate status", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
				return reconcile.Result{}, err
			}
		}
		logger.Info("Could not get gate.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
	} else {
		logger.Info("Gate exists.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
		scope, _ := pgctrl.NewPipelineGateScope(ctx, s.K8sClient, currentGate)

		// if the gate already has a corresponding CR on the cluster, we check if we need to reset the state
		// if the gate is in a terminal state, i.e. open or closed, and that state has been reported, we reset the state to pending
		if (currentGate.Status.IsClosed() || currentGate.Status.IsOpen()) && !currentGate.Status.HasNotReported() {
			logger.Info(fmt.Sprintf("Resetting gate to %s.", gateCR.Status.SyncedState), "Namespace", currentGate.Namespace, "Name", currentGate.Name, "ID", currentGate.Spec.ID)
			currentGate.Spec = gateCR.Spec // this should stay the same, but in case it changes (because we recreated the pipeline alltogether), we need to update it, because the ID would have changed
			currentGate.Status.State = &preserveStatus.SyncedState
			currentGate.Status.SyncedState = preserveStatus.SyncedState
			currentGate.Status.LastSyncedAt = preserveStatus.LastSyncedAt
			currentGate.Status.JobRef = nil

			err = scope.PatchObject()
			if err != nil {
				logger.Error(err, "failed to patch gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
				return reconcile.Result{}, err
			}
		} else {
			if gateCR.Status.IsInitialized() {
				logger.Info(fmt.Sprintf("Gate is in a non-terminal state %s.", *currentGate.Status.State), "Namespace", currentGate.Namespace, "Name", currentGate.Name, "ID", currentGate.Spec.ID)
			} else {
				logger.Info("Gate is not yet initialized.", "Namespace", currentGate.Namespace, "Name", currentGate.Name, "ID", currentGate.Spec.ID)
			}
		}
	}

	return
}

func (s *GateReconciler) CheckNamespace(namespace string) error {
	if namespace == "" {
		return nil
	}
	_, err := s.Clientset.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (s *GateReconciler) GetPublisher() (string, websocket.Publisher) {
	return "gate.event", &socketPublisher{
		gateQueue: s.GateQueue,
		gateCache: s.GateCache,
	}
}
