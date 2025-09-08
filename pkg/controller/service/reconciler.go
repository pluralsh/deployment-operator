package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/common"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	clienterrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/metrics"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/client"
	agentcommon "github.com/pluralsh/deployment-operator/pkg/common"
	common2 "github.com/pluralsh/deployment-operator/pkg/controller/common"
	plrlerrors "github.com/pluralsh/deployment-operator/pkg/errors"
	internallog "github.com/pluralsh/deployment-operator/pkg/log"
	manis "github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/applier"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

const (
	Identifier           = "Service Controller"
	OperatorService      = "deploy-operator"
	RestoreConfigMapName = "restore-config-map"
	// The field manager name for the ones agentk owns, see
	// https://kubernetes.io/docs/reference/using-api/server-side-apply/#field-management
	fieldManager                 = "application/apply-patch"
	IgnoreFieldsAnnotationName   = "deployments.plural.sh/ignore-fields"
	BackFillFieldsAnnotationName = "deployments.plural.sh/backfill-fields"
)

type ServiceReconciler struct {
	consoleClient                                                                  client.Client
	clientset                                                                      *kubernetes.Clientset
	applier                                                                        *applier.Applier
	svcQueue                                                                       workqueue.TypedRateLimitingInterface[string]
	typedRateLimiter                                                               workqueue.TypedRateLimiter[string]
	svcCache                                                                       *client.Cache[console.ServiceDeploymentForAgent]
	manifestCache                                                                  *manis.ManifestCache
	utilFactory                                                                    util.Factory
	restoreNamespace                                                               string
	mapper                                                                         meta.RESTMapper
	k8sClient                                                                      ctrclient.Client
	pollInterval                                                                   time.Duration
	config                                                                         *rest.Config
	dynamicClient                                                                  dynamic.Interface
	store                                                                          store.Store
	refresh, manifestTTL, manifestTTLJitter, workqueueBaseDelay, workqueueMaxDelay time.Duration
	workqueueQPS, workqueueBurst                                                   int
	consoleURL                                                                     string
	waveDelay                                                                      time.Duration
}

func NewServiceReconciler(consoleClient client.Client, k8sClient ctrclient.Client, config *rest.Config, dynamicClient dynamic.Interface, store store.Store, option ...ServiceReconcilerOption) (*ServiceReconciler, error) {
	result := &ServiceReconciler{
		consoleClient:      consoleClient,
		k8sClient:          k8sClient,
		config:             config,
		dynamicClient:      dynamicClient,
		store:              store,
		refresh:            2 * time.Minute,
		manifestTTL:        3 * time.Hour,
		manifestTTLJitter:  30 * time.Minute,
		workqueueBaseDelay: 5 * time.Second,
		workqueueMaxDelay:  300 * time.Second,
		workqueueQPS:       10,
		workqueueBurst:     20,
		waveDelay:          1 * time.Second,
		pollInterval:       2 * time.Minute,
	}

	for _, opt := range option {
		opt(result)
	}

	if result.restoreNamespace == "" {
		return nil, fmt.Errorf("no restore namespace specified")
	}
	if result.consoleURL == "" {
		return nil, fmt.Errorf("no console URL specified")
	}

	// Initialize
	return result.init()
}

func (s *ServiceReconciler) init() (*ServiceReconciler, error) {
	utils.DisableClientLimits(s.config)

	_, deployToken := s.consoleClient.GetCredentials()

	f := utils.NewFactory(s.config)
	mapper, err := f.ToRESTMapper()
	if err != nil {
		return nil, err
	}
	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	// Create a bucket rate limiter
	typedRateLimiter := workqueue.NewTypedMaxOfRateLimiter(workqueue.NewTypedItemExponentialFailureRateLimiter[string](s.workqueueBaseDelay, s.workqueueMaxDelay),
		&workqueue.TypedBucketRateLimiter[string]{Limiter: rate.NewLimiter(rate.Limit(s.workqueueQPS), s.workqueueBurst)},
	)

	return &ServiceReconciler{
		consoleClient:    s.consoleClient,
		clientset:        cs,
		svcQueue:         workqueue.NewTypedRateLimitingQueue(typedRateLimiter),
		typedRateLimiter: typedRateLimiter,
		svcCache: client.NewCache[console.ServiceDeploymentForAgent](s.refresh, func(id string) (
			*console.ServiceDeploymentForAgent, error,
		) {
			return s.consoleClient.GetService(id)
		}),
		manifestCache:    manis.NewCache(s.manifestTTL, s.manifestTTLJitter, deployToken, s.consoleURL),
		applier:          applier.NewApplier(s.dynamicClient, s.store, applier.WithWaveDelay(s.waveDelay), applier.WithFilter(applier.CacheFilter())),
		utilFactory:      f,
		restoreNamespace: s.restoreNamespace,
		mapper:           mapper,
		k8sClient:        s.k8sClient,
		pollInterval:     s.pollInterval,
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
	s.svcQueue = workqueue.NewTypedRateLimitingQueue(s.typedRateLimiter)
}

func (s *ServiceReconciler) Shutdown() {
	s.svcQueue.ShutDown()
	s.svcCache.Wipe()
}

func (s *ServiceReconciler) GetPollInterval() func() time.Duration {
	return func() time.Duration {
		// poll-interval cannot be lower than 10s
		if servicePollInterval := agentcommon.GetConfigurationManager().GetServicePollInterval(); servicePollInterval != nil && *servicePollInterval >= 10*time.Second {
			return *servicePollInterval
		}
		return s.pollInterval
	}
}

func (s *ServiceReconciler) GetPublisher() (string, websocket.Publisher) {
	return "service.event", &socketPublisher{
		svcQueue: s.svcQueue,
		svcCache: s.svcCache,
		manCache: s.manifestCache,
	}
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

func (s *ServiceReconciler) ignoreUpdateFields(ctx context.Context, objs []unstructured.Unstructured, svc *console.ServiceDeploymentForAgent) ([]unstructured.Unstructured, error) {
	normalizerMap := make(map[normalizerKey][]string)

	if svc == nil {
		return objs, nil
	}
	if svc.SyncConfig != nil {
		for _, dn := range svc.SyncConfig.DiffNormalizers {
			kind := lo.FromPtr(dn.Kind)
			name := lo.FromPtr(dn.Name)
			ns := lo.FromPtr(dn.Namespace)
			bf := lo.FromPtr(dn.Backfill)
			key := normalizerKey{Kind: kind, Name: name, Namespace: ns, BackFill: bf}
			for _, ptr := range dn.JSONPointers {
				if ptr != nil && *ptr != "" {
					normalizerMap[key] = append(normalizerMap[key], *ptr)
				}
			}
		}
	}

	for i := range objs {
		obj := objs[i]
		var ignorePaths []string
		var backFillPaths []string
		backFillPaths = getJsonPaths(obj, BackFillFieldsAnnotationName)
		ignorePaths = getJsonPaths(obj, IgnoreFieldsAnnotationName)

		for key, paths := range normalizerMap {
			match, backFill := matchesKey(obj, key)
			if match && !backFill {
				ignorePaths = append(ignorePaths, paths...)
			}
			if match && backFill {
				backFillPaths = append(backFillPaths, paths...)
			}
		}

		if len(backFillPaths) > 0 {
			newObj, err := BackFillJSONPaths(ctx, s.k8sClient, obj, backFillPaths)
			if err != nil {
				return nil, err
			}
			objs[i] = newObj
		}
		if len(ignorePaths) > 0 {
			newObj, err := IgnoreJSONPaths(objs[i], ignorePaths)
			if err != nil {
				return nil, err
			}
			objs[i] = newObj
		}
	}

	return objs, nil
}

func getJsonPaths(obj unstructured.Unstructured, annotation string) []string {
	var ignorePaths []string
	if annotation := obj.GetAnnotations()[annotation]; annotation != "" {
		for _, p := range strings.Split(annotation, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				ignorePaths = append(ignorePaths, p)
			}
		}
	}
	return ignorePaths
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
		annotations[smcommon.LifecycleDeleteAnnotation] = smcommon.PreventDeletion
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
	logger.V(3).Info("fetching services for cluster")

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

	return nil
}

func (s *ServiceReconciler) Reconcile(ctx context.Context, id string) (result reconcile.Result, err error) {
	start := time.Now()
	ctx = context.WithValue(ctx, metrics.ContextKeyTimeStart, start)
	done := false
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
		}

		// Update the error status if the error is not expected or nil (to clear the status).
		if !errors.Is(err, plrlerrors.ErrExpected) && !done {
			s.UpdateErrorStatus(ctx, id, err)
		}

		metrics.Record().ServiceReconciliation(
			id,
			svc.Name,
			metrics.WithServiceReconciliationError(err),
			metrics.WithServiceReconciliationStartedAt(start),
			metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationFinish),
		)
	}()

	logger.V(4).Info("local", "flag", args.Local())
	if args.Local() && svc.Name == OperatorService {
		return
	}

	if svc.DeletedAt != nil {
		logger.V(2).Info("deleting service", "name", svc.Name, "namespace", svc.Namespace)
		components, err := s.applier.Destroy(ctx, id)
		metrics.Record().ServiceDeletion(id)
		s.svcCache.Expire(id)
		s.manifestCache.Expire(id)
		if err != nil {
			logger.Error(err, "failed to update status")
			return ctrl.Result{}, err
		}

		// delete service when components len == 0 (no new statuses, inventory file is empty, all deleted)
		if err := s.UpdateStatus(svc.ID, svc.Revision.ID, svc.Sha, lo.ToSlicePtr(components), []*console.ServiceErrorAttributes{}); err != nil {
			logger.Error(err, "Failed to update service status, ignoring for now")
		}

		err = s.DeleteNamespace(ctx, svc.Namespace, svc.SyncConfig)
		return ctrl.Result{}, err
	}

	logger.V(2).Info("syncing service", "name", svc.Name, "namespace", svc.Namespace)
	logger.V(4).Info("Fetching manifests", "service", svc.Name)
	dir, err := s.manifestCache.Fetch(svc)
	if err != nil {
		logger.Error(err, "failed to parse manifests", "service", svc.Name)
		// mark as the expected error so that it won't get propagated to the API as a service error
		err = plrlerrors.ErrExpected
		return
	}

	manifests, err := template.Render(dir, svc, s.utilFactory)
	if err != nil {
		logger.Error(err, "failed to render manifests", "service", svc.Name)
		return
	}

	manifests = postProcess(manifests)
	metrics.Record().ServiceReconciliation(
		id,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationPrepareManifestsFinish),
	)

	if err = s.CheckNamespace(svc.Namespace, svc.SyncConfig); err != nil {
		logger.Error(err, "failed to check namespace")
		return
	}

	err = s.enforceNamespace(manifests, svc)
	if err != nil {
		return
	}

	manifests, err = s.ignoreUpdateFields(ctx, manifests, svc)
	if err != nil {
		return
	}

	// options := apply.ApplierOptions{
	//	ServerSideOptions: common.ServerSideOptions{
	//		ServerSideApply: true,
	//		ForceConflicts:  true,
	//		FieldManager:    fieldManager,
	//	},
	//	ReconcileTimeout:       10 * time.Second,
	//	EmitStatusEvents:       true,
	//	NoPrune:                false,
	//	DryRunStrategy:         common.DryRunNone,
	//	PrunePropagationPolicy: metav1.DeletePropagationBackground,
	//	PruneTimeout:           20 * time.Second,
	//	InventoryPolicy:        inventory.PolicyAdoptAll,
	//}
	//
	dryRun := false
	if svc.DryRun != nil {
		dryRun = *svc.DryRun
	}
	svc.DryRun = &dryRun

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationApplyStart),
	)

	components, errs, err := s.applier.Apply(ctx,
		*svc,
		manifests,
		applier.WithWaveDryRun(dryRun),
		applier.WithWaveOnApply(func(obj unstructured.Unstructured) {
			gvr := helpers.GVRFromGVK(obj.GroupVersionKind())
			klog.V(internallog.LogLevelDebug).InfoS("registering gvr to watch", "gvr", gvr)
			streamline.GetSupervisor().Register(gvr)
		}),
	)
	if err != nil {
		logger.Error(err, "failed to apply manifests", "service", svc.Name)
		return
	}

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationApplyFinish),
	)

	if err = s.UpdateStatus(svc.ID, svc.Revision.ID, svc.Sha, lo.ToSlicePtr(components), lo.ToSlicePtr(errs)); err != nil {
		logger.Error(err, "Failed to update service status, ignoring for now")
	} else {
		done = true
	}

	metrics.Record().ServiceReconciliation(
		svc.ID,
		svc.Name,
		metrics.WithServiceReconciliationStartedAt(start),
		metrics.WithServiceReconciliationStage(metrics.ServiceReconciliationUpdateStatusFinish),
	)

	return
}

func (s *ServiceReconciler) DeleteNamespace(ctx context.Context, namespace string, syncConfig *console.ServiceDeploymentForAgent_SyncConfig) error {
	deleteNamespace := false
	if syncConfig != nil && syncConfig.DeleteNamespace != nil {
		deleteNamespace = *syncConfig.DeleteNamespace
	}
	if deleteNamespace {
		return utils.DeleteNamespace(ctx, *s.clientset, namespace)
	}
	return nil
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
