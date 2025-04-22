package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
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
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/internal/kubernetes/schema"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	agentcommon "github.com/pluralsh/deployment-operator/pkg/common"
	common2 "github.com/pluralsh/deployment-operator/pkg/controller/common"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

const (
	Identifier           = "Service Controller"
	OperatorService      = "deploy-operator"
	RestoreConfigMapName = "restore-config-map"
	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

type ServiceReconciler struct {
	consoleClient    client.Client
	clientset        *kubernetes.Clientset
	applier          *applier.Applier
	destroyer        *apply.Destroyer
	svcQueue         workqueue.TypedRateLimitingInterface[string]
	svcCache         *client.Cache[console.ServiceDeploymentForAgent]
	manifestCache    *manis.ManifestCache
	utilFactory      util.Factory
	restoreNamespace string
	mapper           meta.RESTMapper
	pinger           *ping.Pinger
	apiExtClient     *apiextensionsclient.Clientset
}

func NewServiceReconciler(consoleClient client.Client, config *rest.Config, refresh, manifestTTL time.Duration, restoreNamespace, consoleURL string) (*ServiceReconciler, error) {
	utils.DisableClientLimits(config)

	_, deployToken := consoleClient.GetCredentials()
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	f := utils.NewFactory(config)
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	apiExtClient, err := apiextensionsclient.NewForConfig(config)
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
		consoleClient: consoleClient,
		clientset:     cs,
		svcQueue:      workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		svcCache: client.NewCache[console.ServiceDeploymentForAgent](refresh, func(id string) (
			*console.ServiceDeploymentForAgent, error,
		) {
			return consoleClient.GetService(id)
		}),
		manifestCache:    manis.NewCache(manifestTTL, deployToken, consoleURL),
		utilFactory:      f,
		applier:          a,
		destroyer:        d,
		pinger:           ping.New(consoleClient, discoveryClient, f),
		restoreNamespace: restoreNamespace,
		mapper:           mapper,
		apiExtClient:     apiExtClient,
	}, nil
}

func (s *ServiceReconciler) Queue() workqueue.TypedRateLimitingInterface[string] {
	return s.svcQueue
}

func (s *ServiceReconciler) Restart() {
	// Cleanup
	s.svcQueue.ShutDown()
	s.svcCache.Wipe()

	// Initialize
	s.svcQueue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
}

func (s *ServiceReconciler) Shutdown() {
	s.svcQueue.ShutDown()
	s.svcCache.Wipe()
}

func (s *ServiceReconciler) GetPollInterval() time.Duration {
	return 0 // use default poll interval
}

func (s *ServiceReconciler) GetPublisher() (string, websocket.Publisher) {
	return "service.event", &socketPublisher{
		svcQueue: s.svcQueue,
		svcCache: s.svcCache,
		manCache: s.manifestCache,
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

func (s *ServiceReconciler) enforceNamespace(objs []unstructured.Unstructured, svc *console.ServiceDeploymentForAgent) error {
	if svc == nil {
		return nil
	}
	if svc.SyncConfig == nil {
		return nil
	}
	if svc.SyncConfig.EnforceNamespace == nil {
		return nil
	}
	if !*svc.SyncConfig.EnforceNamespace {
		return nil
	}

	// find any crds in the set of resources.
	crdObjs := make([]*unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if object.IsCRD(&obj) {
			crdObjs = append(crdObjs, &obj)
		}
	}
	for i := range objs {
		// Look up the scope of the resource so we know if the resource
		// should have a namespace set or not.
		scope, err := object.LookupResourceScope(&objs[i], crdObjs, s.mapper)
		if err != nil {
			return err
		}

		switch scope {
		case meta.RESTScopeNamespace:
			objs[i].SetNamespace(svc.Namespace)
		case meta.RESTScopeRoot:
			return fmt.Errorf("the service %s with 'enforceNamespace' flag has cluster-scoped resources", svc.ID)
		default:
			return fmt.Errorf("unknown RESTScope %q", scope.Name())
		}
	}

	return nil
}

func postProcess(mans []unstructured.Unstructured) []unstructured.Unstructured {
	return lo.Map(mans, func(man unstructured.Unstructured, ind int) unstructured.Unstructured {
		labels := man.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[agentcommon.ManagedByLabel] = agentcommon.AgentLabelValue
		man.SetLabels(labels)
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
	s.svcCache.Wipe()
	s.manifestCache.Wipe()
}

func (s *ServiceReconciler) ShutdownQueue() {
	s.svcQueue.ShutDown()
}

func (s *ServiceReconciler) ListServices(ctx context.Context) *algorithms.Pager[*console.ServiceDeploymentEdgeFragmentForAgent] {
	logger := log.FromContext(ctx)
	logger.Info("create service pager")
	fetch := func(page *string, size int64) ([]*console.ServiceDeploymentEdgeFragmentForAgent, *algorithms.PageInfo, error) {
		resp, err := s.consoleClient.GetServices(page, &size)
		if err != nil {
			logger.Error(err, "failed to fetch service list from deployments service")
			return nil, nil, err
		}
		pageInfo := &algorithms.PageInfo{
			HasNext:  resp.PagedClusterServices.PageInfo.HasNextPage,
			After:    resp.PagedClusterServices.PageInfo.EndCursor,
			PageSize: size,
		}
		return resp.PagedClusterServices.Edges, pageInfo, nil
	}
	return algorithms.NewPager[*console.ServiceDeploymentEdgeFragmentForAgent](common2.DefaultPageSize, fetch)
}

func (s *ServiceReconciler) Poll(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("fetching services for cluster")

	restore, err := s.isClusterRestore(ctx)
	if err != nil {
		logger.Error(err, "failed to check restore config map")
		return err
	}
	if restore {
		logger.Info("restoring cluster from backup")
		return nil
	}

	pager := s.ListServices(ctx)

	for pager.HasNext() {
		services, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch service list from deployments service")
			return err
		}
		for _, svc := range services {
			// If services arg is provided, we can skip
			// services that are not on the list.
			if args.SkipService(svc.Node.ID) {
				continue
			}

			logger.V(4).Info("sending update for", "service", svc.Node.ID)
			s.svcCache.Add(svc.Node.ID, svc.Node)
			s.svcQueue.Add(svc.Node.ID)
		}
	}

	if err := s.pinger.Ping(); err != nil {
		logger.Error(err, "failed to ping cluster after scheduling syncs")
	}

	s.ScrapeKube(ctx)
	return nil
}

func (s *ServiceReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	start := time.Now()
	ctx = context.WithValue(ctx, metrics.ContextKeyTimeStart, start)

	logger := log.FromContext(ctx)
	logger.V(4).Info("attempting to sync service", "id", id)

	svc, err := s.svcCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.V(4).Info("service already deleted", "id", id)
			return reconcile.Result{}, nil
		}
		logger.Error(err, fmt.Sprintf("failed to fetch service: %s, ignoring for now", id))
		return
	}

	metrics.Record().ServiceReconciliation(
		id,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationStart),
	)

	defer func() {
		if err != nil {
			logger.Error(err, "process item")
			if !errors.Is(err, plrlerrors.ErrExpected) {
				s.UpdateErrorStatus(ctx, id, err)
			}
		}

		metrics.Record().ServiceReconciliation(
			id,
			svc.Name,
			metrics.WithServiceReconciliationError(err),
			metrics.WithServiceReconciliationStartedAt(start),
			metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationFinish),
		)
	}()

	logger.V(2).Info("local", "flag", args.Local())
	if args.Local() && svc.Name == OperatorService {
		return
	}

	logger.V(2).Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	if svc.DeletedAt != nil {
		logger.V(2).Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := s.destroyer.Run(ctx, inventory.WrapInventoryInfoObj(lo.ToPtr(s.defaultInventoryObjTemplate(id))), apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationBackground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})

		metrics.Record().ServiceDeletion(id)
		s.svcCache.Expire(id)
		s.manifestCache.Expire(id)
		err = s.UpdatePruneStatus(ctx, svc, ch, map[schema.GroupName]string{})
		return
	}

	logger.V(4).Info("Fetching manifests", "service", svc.Name)
	manifests, err := s.manifestCache.Fetch(s.utilFactory, svc)
	if err != nil {
		logger.Error(err, "failed to parse manifests", "service", svc.Name)
		return
	}
	manifests = postProcess(manifests)
	logger.V(4).Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := s.SplitObjects(id, manifests)
	if err != nil {
		return
	}
	inv := inventory.WrapInventoryInfoObj(&invObj)

	metrics.Record().ServiceReconciliation(
		id,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationPrepareManifestsFinish),
	)

	vcache := manis.VersionCache(manifests)

	if err = s.CheckNamespace(svc.Namespace, svc.SyncConfig); err != nil {
		logger.Error(err, "failed to check namespace")
		return
	}

	err = s.enforceNamespace(manifests, svc)
	if err != nil {
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

	ch := s.applier.Run(ctx, inv, manifests, options)
	err = s.UpdateApplyStatus(ctx, svc, ch, false, vcache)

	return
}

func (s *ServiceReconciler) CheckNamespace(namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error {
	createNamespace := true
	var labels map[string]string
	var annotations map[string]string

	if syncConfig != nil {
		if syncConfig.NamespaceMetadata != nil {
			labels = utils.ConvertMap(syncConfig.NamespaceMetadata.Labels)
			annotations = utils.ConvertMap(syncConfig.NamespaceMetadata.Annotations)
		}
		if syncConfig.CreateNamespace != nil {
			createNamespace = *syncConfig.CreateNamespace
		}
	}
	if createNamespace {
		return utils.CheckNamespace(*s.clientset, namespace, labels, annotations)
	}
	return nil
}

func (s *ServiceReconciler) SplitObjects(id string, objs []unstructured.Unstructured) (unstructured.Unstructured, []unstructured.Unstructured, error) {
	invs := make([]unstructured.Unstructured, 0, 1)
	resources := make([]unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		if inventory.IsInventoryObject(&obj) {
			invs = append(invs, obj)
		} else {
			resources = append(resources, obj)
		}
	}
	switch len(invs) {
	case 0:
		return s.defaultInventoryObjTemplate(id), resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return unstructured.Unstructured{}, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func (s *ServiceReconciler) defaultInventoryObjTemplate(id string) unstructured.Unstructured {
	name := "inventory-" + id
	namespace := "plrl-deploy-operator"

	return unstructured.Unstructured{
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
	}
}

func (s *ServiceReconciler) isClusterRestore(ctx context.Context) (bool, error) {
	cmr, err := s.clientset.CoreV1().ConfigMaps(s.restoreNamespace).Get(ctx, RestoreConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, nil
	}
	timeout := cmr.CreationTimestamp.Add(time.Hour)
	if time.Now().After(timeout) {
		if err := s.clientset.CoreV1().ConfigMaps(s.restoreNamespace).Delete(ctx, RestoreConfigMapName, metav1.DeleteOptions{}); err != nil {
			return true, err
		}
		return false, nil
	}
	return true, nil
}
