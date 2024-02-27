package pipelinegates

import (
	"context"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/api/v1alpha1"
	pgctrl "github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type GateReconciler struct {
	K8sClient         ctrlclient.Client
	ConsoleClient     client.Client
	Config            *rest.Config
	Clientset         *kubernetes.Clientset
	GateCache         *client.Cache[console.PipelineGateFragment]
	GateQueue         workqueue.RateLimitingInterface
	UtilFactory       util.Factory
	discoveryClient   *discovery.DiscoveryClient
	pinger            *ping.Pinger
	operatorNamespace string
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

	namespace, err := utils.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}
	return &GateReconciler{
		K8sClient:         k8sClient,
		ConsoleClient:     consoleClient,
		Config:            config,
		Clientset:         cs,
		GateQueue:         gateQueue,
		GateCache:         gateCache,
		UtilFactory:       f,
		discoveryClient:   discoveryClient,
		pinger:            ping.New(consoleClient, discoveryClient, f),
		operatorNamespace: namespace,
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
	logger.V(1).Info("fetching gates for cluster")

	var after *string
	var pageSize int64
	pageSize = 100
	hasNextPage := true

	for hasNextPage {
		resp, err := s.ConsoleClient.GetClusterGates(after, &pageSize)
		if err != nil {
			logger.Error(err, "failed to fetch gates list")
			return false, nil
		}

		hasNextPage = resp.PagedClusterGates.PageInfo.HasNextPage
		after = resp.PagedClusterGates.PageInfo.EndCursor

		for _, gate := range resp.PagedClusterGates.Edges {
			logger.V(2).Info("sending update for", "gate", gate.Node.ID)
			s.GateQueue.Add(gate.Node.ID)
		}
	}

	if err := s.pinger.Ping(); err != nil {
		logger.Error(err, "failed to ping cluster after scheduling syncs")
	}

	return false, nil
}

func (s *GateReconciler) Reconcile(ctx context.Context, id string) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	logger.V(1).Info("attempting to sync gate", "id", id)
	var gate *console.PipelineGateFragment
	gate, err := s.GateCache.Get(id)
	if err != nil {
		logger.Error(err, "failed to fetch gate: %s, ignoring for now")
		return reconcile.Result{}, err
	}

	logger.V(1).Info("attempting to sync gate", "Name", gate.Name, "ID", gate.ID)

	if gate.Type != console.GateTypeJob {
		logger.V(1).Info(fmt.Sprintf("gate is of type %s, we only reconcile gates of type %s skipping", gate.Type, console.GateTypeJob), "Name", gate.Name, "ID", gate.ID)
		return reconcile.Result{}, nil
	}

	gateCR, err := s.ConsoleClient.ParsePipelineGateCR(gate, s.operatorNamespace)
	if err != nil {
		logger.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
		return reconcile.Result{}, err
	}

	// get pipelinegate
	currentGate := &v1alpha1.PipelineGate{}
	if err := s.K8sClient.Get(ctx, types.NamespacedName{Name: gateCR.Name, Namespace: gateCR.Namespace}, currentGate); err != nil {
		if !apierrors.IsNotFound(err) {
			logger.V(1).Info("Could not get gate.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
			return reconcile.Result{}, err
		}

		logger.V(1).Info("This gate doesn't yet have a corresponding CR on this cluster yet.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
		// If the PipelineGate doesn't exist, create it.
		if err = s.K8sClient.Create(context.Background(), gateCR); err != nil {
			logger.Error(err, "failed to create gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	logger.V(1).Info("Gate exists.", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
	scope, err := pgctrl.NewPipelineGateScope(ctx, s.K8sClient, currentGate)
	if err != nil {
		return reconcile.Result{}, err
	}

	currentGate.Spec = gateCR.Spec
	err = scope.PatchObject()
	if err != nil {
		logger.Error(err, "failed to patch gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (s *GateReconciler) GetPublisher() (string, websocket.Publisher) {
	return "gate.event", &socketPublisher{
		gateQueue: s.GateQueue,
		gateCache: s.GateCache,
	}
}
