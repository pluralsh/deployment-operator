package pipelinegates

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	generated "github.com/pluralsh/deployment-operator/generated/client/clientset/versioned"
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller/service"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func init() {
	Local = false
}

var (
	Local = false
)

const (
	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

type GateReconciler struct {
	ConsoleClient   *client.Client
	Config          *rest.Config
	Clientset       *kubernetes.Clientset
	GenClientset    *generated.Clientset
	GateCache       *client.Cache[console.PipelineGateFragment]
	GateQueue       workqueue.RateLimitingInterface
	UtilFactory     util.Factory
	discoveryClient *discovery.DiscoveryClient
	pinger          *ping.Pinger
}

func NewGateReconciler(consoleClient *client.Client, config *rest.Config, refresh time.Duration, clusterId string) (*GateReconciler, error) {
	service.DisableClientLimits(config)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	gateCache := client.NewCache[console.PipelineGateFragment](refresh, func(id string) (*console.PipelineGateFragment, error) {
		return consoleClient.GetClusterGate(id)
	})

	gateQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	f := service.NewFactory(config)

	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	genClientset, err := generated.NewForConfig(config)
	return &GateReconciler{
		ConsoleClient:   consoleClient,
		Config:          config,
		Clientset:       cs,
		GenClientset:    genClientset,
		GateQueue:       gateQueue,
		GateCache:       gateCache,
		UtilFactory:     f,
		discoveryClient: discoveryClient,
		pinger:          ping.New(consoleClient, discoveryClient, f),
	}, nil
}

func newApplier(invFactory inventory.ClientFactory, f util.Factory) (*applier.Applier, error) {
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return nil, err
	}

	return applier.NewApplierBuilder().
		WithFactory(f).
		WithInventoryClient(invClient).
		Build()
}

func newDestroyer(invFactory inventory.ClientFactory, f util.Factory) (*apply.Destroyer, error) {
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return nil, err
	}

	return apply.NewDestroyerBuilder().
		WithFactory(f).
		WithInventoryClient(invClient).
		Build()
}

func postProcess(mans []*unstructured.Unstructured) []*unstructured.Unstructured {
	return lo.Map(mans, func(man *unstructured.Unstructured, ind int) *unstructured.Unstructured {
		if man.GetKind() != "CustomResourceDefinition" {
			return man
		}

		annotations := man.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[common.LifecycleDeleteAnnotation] = common.PreventDeletion
		man.SetAnnotations(annotations)
		return man
	})
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
		fmt.Printf("failed to fetch gate: %s, ignoring for now", err)
		return
	}

	defer func() {
		if err != nil {
			logger.Error(err, "failed to reconcile gate", "Name", gate.Name, "Id", gate.ID)
		}
	}()

	logger.Info("attempting to sync gate", "Name", gate.Name, "ID", gate.ID)

	//TODO: DEBUG
	gateJSON, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		logger.Error(err, "failed to marshalindent gate")
	}
	fmt.Printf("gate json from API: \n %s\n", string(gateJSON))
	//TODO: DEBUG

	if gate.Type != console.GateTypeJob {
		logger.Info(fmt.Sprintf("gate is of type %s, we only reconcile gates of type %s skipping", gate.Type, console.GateTypeJob), "Name", gate.Name, "ID", gate.ID)
	}

	gateCR, err := s.ConsoleClient.ParsePipelineGateCR(gate)
	if err != nil {
		logger.Error(err, "failed to parse gate CR", "Name", gate.Name, "ID", gate.ID)
		return reconcile.Result{}, err
	}

	//TODO: DEBUG
	gateCRJSON, err := json.MarshalIndent(gateCR, "", "  ")
	if err != nil {
		logger.Error(err, "failed to marshalindent gateCR")
	}
	fmt.Printf("updating or creating gateCR json:\n %s\n", string(gateCRJSON))
	//TODO: DEBUG

	pgClient := s.GenClientset.PipelinesV1alpha1().PipelineGates(gateCR.Namespace)
	patchData, _ := json.Marshal(gateCR)
	_, err = pgClient.Patch(context.Background(), gateCR.Name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")
	//pgClient.Patch(context.Background(), gateCR.Name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")
	//_, err = pgClient.Patch(context.Background(), gateCR.Name, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the PipelineGate doesn't exist, create it.
			_, err = pgClient.Create(context.Background(), gateCR, metav1.CreateOptions{})
			if err != nil {
				logger.Error(err, "failed to create gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
				return reconcile.Result{}, err
			}
		} else {
			logger.Error(err, "failed to patch gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
			return reconcile.Result{}, err
		}
	}
	logger.Info("Patched pipeline gate", "Namespace", gateCR.Namespace, "Name", gateCR.Name, "ID", gateCR.Spec.ID)
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

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (s *GateReconciler) GetPublisher() (string, websocket.Publisher) {
	return "gate.event", &socketPublisher{
		gateQueue: s.GateQueue,
		gateCache: s.GateCache,
	}
}
