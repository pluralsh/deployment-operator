package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

func init() {
	Local = false
}

var (
	Local = false
)

const (
	OperatorService = "deploy-operator"

	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

type ServiceReconciler struct {
	ConsoleClient client.Client
	Config        *rest.Config
	Clientset     *kubernetes.Clientset
	Applier       *applier.Applier
	Destroyer     *apply.Destroyer
	SvcQueue      workqueue.RateLimitingInterface
	SvcCache      *client.Cache[console.GetServiceDeploymentForAgent_ServiceDeployment]
	ManifestCache *manifests.ManifestCache
	UtilFactory   util.Factory
	LuaScript     string

	discoveryClient *discovery.DiscoveryClient
	pinger          *ping.Pinger
}

func NewServiceReconciler(consoleClient client.Client, config *rest.Config, refresh time.Duration) (*ServiceReconciler, error) {
	utils.DisableClientLimits(config)

	_, deployToken := consoleClient.GetCredentials()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	svcCache := client.NewCache[console.GetServiceDeploymentForAgent_ServiceDeployment](refresh, func(id string) (*console.GetServiceDeploymentForAgent_ServiceDeployment, error) {
		return consoleClient.GetService(id)
	})

	svcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	manifestCache := manifests.NewCache(refresh, deployToken)

	f := utils.NewFactory(config)

	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	invFactory := inventory.ClusterClientFactory{StatusPolicy: inventory.StatusPolicyNone}

	a, err := newApplier(invFactory, f)
	if err != nil {
		return nil, err
	}

	d, err := newDestroyer(invFactory, f)
	if err != nil {
		return nil, err
	}

	return &ServiceReconciler{
		ConsoleClient:   consoleClient,
		Config:          config,
		Clientset:       cs,
		SvcQueue:        svcQueue,
		SvcCache:        svcCache,
		ManifestCache:   manifestCache,
		UtilFactory:     f,
		Applier:         a,
		Destroyer:       d,
		discoveryClient: discoveryClient,
		pinger:          ping.New(consoleClient, discoveryClient, f),
	}, nil
}

func (s *ServiceReconciler) GetPublisher() (string, websocket.Publisher) {
	return "service.event", &socketPublisher{
		svcQueue: s.SvcQueue,
		svcCache: s.SvcCache,
		manCache: s.ManifestCache,
	}

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

func (s *ServiceReconciler) WipeCache() {
	s.SvcCache.Wipe()
	s.ManifestCache.Wipe()
}

func (s *ServiceReconciler) ShutdownQueue() {
	s.SvcQueue.ShutDown()
}

func (s *ServiceReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)
	logger.Info("fetching services for cluster")
	var after *string
	var pageSize int64
	pageSize = 100
	hasNextPage := true

	for hasNextPage {
		resp, err := s.ConsoleClient.GetServices(after, &pageSize)
		if err != nil {
			logger.Error(err, "failed to fetch service list from deployments service")
			return false, nil
		}

		hasNextPage = resp.PagedClusterServices.PageInfo.HasNextPage
		after = resp.PagedClusterServices.PageInfo.EndCursor
		for _, svc := range resp.PagedClusterServices.Edges {
			logger.Info("sending update for", "service", svc.Node.ID)
			s.SvcQueue.Add(svc.Node.ID)
		}
	}

	if err := s.pinger.Ping(); err != nil {
		logger.Error(err, "failed to ping cluster after scheduling syncs")
	}

	s.ScrapeKube(ctx)
	return false, nil
}

func (s *ServiceReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	logger := log.FromContext(ctx)

	logger.Info("attempting to sync service", "id", id)
	svc, err := s.SvcCache.Get(id)
	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to fetch service: %s, ignoring for now", id))
		return
	}

	defer func() {
		if err != nil {
			logger.Error(err, "process item")
			if !errors.Is(err, plrlerrors.ErrExpected) {
				s.UpdateErrorStatus(ctx, id, err)
			}
		}
	}()

	logger.Info("local", "flag", Local)
	if Local && svc.Name == OperatorService {
		return
	}

	logger.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	manifests, err := s.ManifestCache.Fetch(s.UtilFactory, svc)
	if err != nil {
		logger.Error(err, "failed to parse manifests")
		return
	}

	manifests = postProcess(manifests)

	logger.Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := s.SplitObjects(id, manifests)
	if err != nil {
		return
	}
	inv := inventory.WrapInventoryInfoObj(invObj)

	vcache := manis.VersionCache(manifests)

	if svc.DeletedAt != nil {
		logger.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := s.Destroyer.Run(ctx, inv, apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationBackground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})

		err = s.UpdatePruneStatus(ctx, svc, ch, vcache)
		return
	}

	logger.Info("Apply service", "name", svc.Name, "namespace", svc.Namespace)
	if err = s.CheckNamespace(svc.Namespace); err != nil {
		logger.Error(err, "failed to check namespace")
		return
	}

	options := apply.ApplierOptions{
		ServerSideOptions: common.ServerSideOptions{
			ServerSideApply: true,
			ForceConflicts:  true,
			FieldManager:    fieldManager,
		},
		ReconcileTimeout:       10 * time.Second,
		EmitStatusEvents:       true,
		NoPrune:                false,
		DryRunStrategy:         common.DryRunNone,
		PrunePropagationPolicy: metav1.DeletePropagationBackground,
		PruneTimeout:           20 * time.Second,
		InventoryPolicy:        inventory.PolicyAdoptAll,
	}

	dryRun := false
	if svc.DryRun != nil {
		dryRun = *svc.DryRun
	}
	svc.DryRun = &dryRun
	if dryRun {
		options.DryRunStrategy = common.DryRunServer
	}

	ch := s.Applier.Run(ctx, inv, manifests, options)
	err = s.UpdateApplyStatus(ctx, svc, ch, false, vcache)

	return
}

func (s *ServiceReconciler) CheckNamespace(namespace string) error {
	return utils.CheckNamespace(*s.Clientset, namespace)
}

func (s *ServiceReconciler) SplitObjects(id string, objs []*unstructured.Unstructured) (*unstructured.Unstructured, []*unstructured.Unstructured, error) {
	invs := make([]*unstructured.Unstructured, 0, 1)
	resources := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if inventory.IsInventoryObject(obj) {
			invs = append(invs, obj)
		} else {
			resources = append(resources, obj)
		}
	}
	switch len(invs) {
	case 0:
		invObj, err := s.defaultInventoryObjTemplate(id)
		if err != nil {
			return nil, nil, err
		}
		return invObj, resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return nil, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func (s *ServiceReconciler) defaultInventoryObjTemplate(id string) (*unstructured.Unstructured, error) {
	name := "inventory-" + id
	namespace := "plrl-deploy-operator"

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					common.InventoryLabel: id,
				},
			},
		},
	}, nil
}

func (s *ServiceReconciler) GetLuaScript() string {
	return s.LuaScript
}

func (s *ServiceReconciler) SetLuaScript(script string) {
	s.LuaScript = script
}
