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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	agentcommon "github.com/pluralsh/deployment-operator/pkg/common"

	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/controller"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
	"github.com/pluralsh/deployment-operator/pkg/ping"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

const (
	OperatorService      = "deploy-operator"
	RestoreConfigMapName = "restore-config-map"
	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager = "application/apply-patch"
)

type ServiceReconciler struct {
	ConsoleClient    client.Client
	Config           *rest.Config
	Clientset        *kubernetes.Clientset
	Applier          *applier.Applier
	Destroyer        *apply.Destroyer
	SvcQueue         workqueue.RateLimitingInterface
	SvcCache         *client.Cache[console.GetServiceDeploymentForAgent_ServiceDeployment]
	ManifestCache    *manifests.ManifestCache
	UtilFactory      util.Factory
	RestoreNamespace string

	mapper          meta.RESTMapper
	discoveryClient *discovery.DiscoveryClient
	pinger          *ping.Pinger
	ctx             context.Context
}

func NewServiceReconciler(ctx context.Context, consoleClient client.Client, config *rest.Config, refresh, manifestTTL time.Duration, restoreNamespace, consoleURL string) (*ServiceReconciler, error) {
	logger := log.FromContext(ctx)
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

	manifestCache := manifests.NewCache(manifestTTL, deployToken, consoleURL)

	f := utils.NewFactory(config)
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}
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

	_ = helpers.BackgroundPollUntilContextCancel(ctx, 5*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		if err = CapabilitiesAPIVersions(discoveryClient); err != nil {
			logger.Error(err, "can't fetch API versions")
		}

		metrics.Record().DiscoveryAPICacheRefresh(err)
		return false, nil
	})

	return &ServiceReconciler{
		ConsoleClient:    consoleClient,
		Config:           config,
		Clientset:        cs,
		SvcQueue:         svcQueue,
		SvcCache:         svcCache,
		ManifestCache:    manifestCache,
		UtilFactory:      f,
		Applier:          a,
		Destroyer:        d,
		discoveryClient:  discoveryClient,
		pinger:           ping.New(consoleClient, discoveryClient, f),
		RestoreNamespace: restoreNamespace,
		ctx:              ctx,
		mapper:           mapper,
	}, nil
}

func CapabilitiesAPIVersions(discoveryClient *discovery.DiscoveryClient) error {
	lists, err := discoveryClient.ServerPreferredResources()

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			template.APIVersions.Set(fmt.Sprintf("%s/%s", gv.String(), resource.Kind), true)
		}
	}
	return err
}

func (s *ServiceReconciler) GetPollInterval() time.Duration {
	return 0 // use default poll interval
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

func (s *ServiceReconciler) enforceNamespace(objs []*unstructured.Unstructured, svc *console.GetServiceDeploymentForAgent_ServiceDeployment) error {
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

	var crdObjs []*unstructured.Unstructured
	// find any crds in the set of resources.
	for _, obj := range objs {
		if object.IsCRD(obj) {
			crdObjs = append(crdObjs, obj)
		}
	}
	for _, obj := range objs {

		// Look up the scope of the resource so we know if the resource
		// should have a namespace set or not.
		scope, err := object.LookupResourceScope(obj, crdObjs, s.mapper)
		if err != nil {
			return err
		}

		switch scope {
		case meta.RESTScopeNamespace:
			obj.SetNamespace(svc.Namespace)
		case meta.RESTScopeRoot:
			return fmt.Errorf("the service %s with 'enforceNamespace' flag has cluster-scoped resources", svc.ID)
		default:
			return fmt.Errorf("unknown RESTScope %q", scope.Name())
		}
	}

	return nil
}

func postProcess(mans []*unstructured.Unstructured) []*unstructured.Unstructured {
	return lo.Map(mans, func(man *unstructured.Unstructured, ind int) *unstructured.Unstructured {
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
	s.SvcCache.Wipe()
	s.ManifestCache.Wipe()
}

func (s *ServiceReconciler) ShutdownQueue() {
	s.SvcQueue.ShutDown()
}

func (s *ServiceReconciler) ListServices(ctx context.Context) *algorithms.Pager[*console.ServiceDeploymentEdgeFragment] {
	logger := log.FromContext(ctx)
	logger.Info("create service pager")
	fetch := func(page *string, size int64) ([]*console.ServiceDeploymentEdgeFragment, *algorithms.PageInfo, error) {
		resp, err := s.ConsoleClient.GetServices(page, &size)
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
	return algorithms.NewPager[*console.ServiceDeploymentEdgeFragment](controller.DefaultPageSize, fetch)
}

func (s *ServiceReconciler) Poll(ctx context.Context) (done bool, err error) {
	logger := log.FromContext(ctx)
	logger.Info("fetching services for cluster")

	restore, err := s.isClusterRestore(ctx)
	if err != nil {
		logger.Error(err, "failed to check restore config map")
		return false, nil
	}
	if restore {
		logger.Info("restoring cluster from backup")
		return false, nil
	}

	pager := s.ListServices(ctx)

	for pager.HasNext() {
		services, err := pager.NextPage()
		if err != nil {
			logger.Error(err, "failed to fetch service list from deployments service")
			return false, nil
		}
		for _, svc := range services {
			// If services arg is provided, we can skip
			// services that are not on the list.
			if args.SkipService(svc.Node.ID) {
				continue
			}

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
	start := time.Now()
	ctx = context.WithValue(ctx, metrics.ContextKeyTimeStart, start)

	logger := log.FromContext(ctx)
	logger.Info("attempting to sync service", "id", id)

	svc, err := s.SvcCache.Get(id)
	if err != nil {
		if clienterrors.IsNotFound(err) {
			logger.Info("service already deleted", "id", id)
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

	logger.Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)

	if svc.DeletedAt != nil {
		logger.Info("Deleting service", "name", svc.Name, "namespace", svc.Namespace)
		ch := s.Destroyer.Run(ctx, inventory.WrapInventoryInfoObj(s.defaultInventoryObjTemplate(id)), apply.DestroyerOptions{
			InventoryPolicy:         inventory.PolicyAdoptIfNoInventory,
			DryRunStrategy:          common.DryRunNone,
			DeleteTimeout:           20 * time.Second,
			DeletePropagationPolicy: metav1.DeletePropagationBackground,
			EmitStatusEvents:        true,
			ValidationPolicy:        1,
		})

		metrics.Record().ServiceDeletion(id)
		err = s.UpdatePruneStatus(ctx, svc, ch, map[manis.GroupName]string{})
		return
	}

	logger.Info("Fetching manifests", "service", svc.Name)
	manifests, err := s.ManifestCache.Fetch(s.UtilFactory, svc)
	if err != nil {
		logger.Error(err, "failed to parse manifests", "service", svc.Name)
		return
	}
	manifests = postProcess(manifests)
	logger.Info("Syncing manifests", "count", len(manifests))
	invObj, manifests, err := s.SplitObjects(id, manifests)
	if err != nil {
		return
	}
	inv := inventory.WrapInventoryInfoObj(invObj)

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

	ch := s.Applier.Run(ctx, inv, manifests, options)
	err = s.UpdateApplyStatus(ctx, svc, ch, false, vcache)

	return
}

func (s *ServiceReconciler) CheckNamespace(namespace string, syncConfig *console.GetServiceDeploymentForAgent_ServiceDeployment_SyncConfig) error {
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
		return utils.CheckNamespace(*s.Clientset, namespace, labels, annotations)
	}
	return nil
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
		return s.defaultInventoryObjTemplate(id), resources, nil
	case 1:
		return invs[0], resources, nil
	default:
		return nil, nil, fmt.Errorf("expecting zero or one inventory object, found %d", len(invs))
	}
}

func (s *ServiceReconciler) defaultInventoryObjTemplate(id string) *unstructured.Unstructured {
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
	}
}

func (s *ServiceReconciler) isClusterRestore(ctx context.Context) (bool, error) {
	cmr, err := s.Clientset.CoreV1().ConfigMaps(s.RestoreNamespace).Get(ctx, RestoreConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, nil
	}
	timeout := cmr.CreationTimestamp.Add(time.Hour)
	if time.Now().After(timeout) {
		if err := s.Clientset.CoreV1().ConfigMaps(s.RestoreNamespace).Delete(ctx, RestoreConfigMapName, metav1.DeleteOptions{}); err != nil {
			return true, err
		}
		return false, nil
	}
	return true, nil
}
