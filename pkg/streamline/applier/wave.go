package applier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/pkg/manifests/template"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/workqueue"

	consoleclient "github.com/pluralsh/deployment-operator/pkg/client"

	"github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	smcommon "github.com/pluralsh/deployment-operator/pkg/streamline/common"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

type WaveType string

const (
	ApplyWave   WaveType = "apply"
	DeleteWave  WaveType = "delete"
	WaitCRDWave WaveType = "waitCRD"
)

// Wave is a collection of resources that will be applied or deleted together.
// It is used to group resources that are related to each other.
type Wave struct {
	// items is the list of resources in the wave
	items []unstructured.Unstructured

	// waveType is the type of the wave
	waveType WaveType
}

func NewWave(resources []unstructured.Unstructured, waveType WaveType) Wave {
	result := Wave{
		items:    resources,
		waveType: waveType,
	}

	return result
}

func (in *Wave) Add(resource unstructured.Unstructured) {
	in.items = append(in.items, resource)
}

func (in *Wave) Len() int {
	return len(in.items)
}

type Waves []Wave

func NewWaves(resources []unstructured.Unstructured) Waves {
	defaultWaveCount := 6
	waves := make([]Wave, 0, defaultWaveCount)
	for i := 0; i < defaultWaveCount; i++ {
		waves = append(waves, NewWave([]unstructured.Unstructured{}, ApplyWave))
	}

	kindToWave := map[string]int{
		// Wave 0 - core non-namespaced resources
		common.NamespaceKind:                0,
		common.CustomResourceDefinitionKind: 0,
		common.PersistentVolumeKind:         0,
		common.ClusterRoleKind:              0,
		common.ClusterRoleListKind:          0,
		common.ClusterRoleBindingKind:       0,
		common.ClusterRoleBindingListKind:   0,
		common.StorageClassKind:             0,

		// Wave 1 - core namespaced configuration resources
		common.ConfigMapKind:             1,
		common.SecretKind:                1,
		common.SecretListKind:            1,
		common.ServiceAccountKind:        1,
		common.RoleKind:                  1,
		common.RoleListKind:              1,
		common.RoleBindingKind:           1,
		common.RoleBindingListKind:       1,
		common.PodDisruptionBudgetKind:   1,
		common.ResourceQuotaKind:         1,
		common.NetworkPolicyKind:         1,
		common.LimitRangeKind:            1,
		common.PodSecurityPolicyKind:     1,
		common.IngressClassKind:          1,
		common.PersistentVolumeClaimKind: 1,

		// Wave 2 - core namespaced workload resources
		common.DeploymentKind:            2,
		common.DaemonSetKind:             2,
		common.StatefulSetKind:           2,
		common.ReplicaSetKind:            2,
		common.JobKind:                   2,
		common.CronJobKind:               2,
		common.PodKind:                   2,
		common.ReplicationControllerKind: 2,

		// Wave 3 - core namespaced networking resources
		common.EndpointsKind:  3,
		common.ServiceKind:    3,
		common.IngressKind:    3,
		common.APIServiceKind: 3,
	}
	waves[4].waveType = WaitCRDWave
	for _, resource := range resources {
		if waveIdx, exists := kindToWave[resource.GetKind()]; exists {
			waves[waveIdx].Add(resource)
		} else {
			// Unknown resource kind, put it in the last wave
			waves[len(waves)-1].Add(resource)
		}
		if template.IsCRD(&resource) {
			waves[4].Add(resource)
		}
	}

	return waves
}

const (
	defaultMaxConcurrentApplies = 10
	defaultDeQueueDelay         = 100 * time.Millisecond
)

// WaveProcessor processes a wave of resources. It applies or deletes the resources in the wave.
// It uses a work queue to process the items in the wave concurrently. It uses a channel to communicate
// between the workers and the collector goroutine. The collector goroutine collects the components and errors
// from the channels and returns them to the caller.
type WaveProcessor struct {
	mu sync.Mutex

	// client is the dynamic client used to apply the resources.
	client dynamic.Interface

	discoveryClient discovery.DiscoveryInterface

	// wave to be processed. It contains the resources to be applied or deleted.
	wave Wave

	// componentChan is used to communicate between the workers and the collector goroutine
	// when a resource is successfully applied, the worker sends the component attributes to the channel.
	componentChan chan console.ComponentAttributes

	// errorsChan is used to communicate between the workers and the collector goroutine
	// when a resource fails to be applied, the worker sends the error to the channel.
	errorsChan chan console.ServiceErrorAttributes

	// queue is the work queue used to process the items in the wave.
	queue *workqueue.Typed[smcommon.Key]

	// keyToResource is a map of the wave items to their keys.
	// It is used to lookup the resource from the key when processing the items in the queue.
	keyToResource map[smcommon.Key]unstructured.Unstructured

	// maxConcurrentApplies is the maximum number of workers that can be started.
	maxConcurrentApplies int

	// concurrentApplies is the number of workers that will be started.
	// It is calculated based on the number of items in the wave and the maxConcurrentApplies option.
	// If the wave contains more items than the maxConcurrentApplies option, the number of workers
	// will be set to the maxConcurrentApplies otherwise it will be set to the number of items in the wave.
	concurrentApplies int

	// deQueueDelay is the amount of time to wait before dequeuing the next item from the queue
	// by the same worker.
	deQueueDelay time.Duration

	// dryRun determines if the wave should be applied in dry run mode, meaning that the changes will not be persisted.
	dryRun bool

	// onApplyCallback is a callback function called when a resource is applied
	onApplyCallback func(resource unstructured.Unstructured)

	// svcCache is the cache used to get the service deployment for an agent.
	svcCache *consoleclient.Cache[console.ServiceDeploymentForAgent]
}

func (in *WaveProcessor) Run(ctx context.Context) (components []console.ComponentAttributes, errors []console.ServiceErrorAttributes) {
	in.mu.Lock()
	defer in.mu.Unlock()
	now := time.Now()

	in.init()

	workerWG := &sync.WaitGroup{}
	collectorWG := &sync.WaitGroup{}

	workerWG.Add(in.concurrentApplies)
	in.runWorkers(ctx, func() { workerWG.Done() })

	collectorWG.Add(1)
	cmpChan := in.componentChan
	errChan := in.errorsChan

	// run a collector goroutine to collect components and errors from the channels
	go func() {
		defer collectorWG.Done()
		for cmpChan != nil || errChan != nil {
			select {
			case <-ctx.Done():
				return
			case component, ok := <-cmpChan:
				if !ok {
					klog.V(log.LogLevelTrace).InfoS("component channel closed")
					cmpChan = nil
					continue
				}

				klog.V(log.LogLevelDebug).InfoS("received component", "component", component)
				components = append(components, component)
			case err, ok := <-errChan:
				if !ok {
					klog.V(log.LogLevelTrace).InfoS("error channel closed")
					errChan = nil
					continue
				}

				klog.V(log.LogLevelDebug).InfoS("received error", "error", err)
				errors = append(errors, err)
			}
		}
	}()

	// no more items will be added, allow workers to drain and exit
	in.queue.ShutDown()

	workerWG.Wait()
	close(in.componentChan)
	close(in.errorsChan)
	collectorWG.Wait()

	klog.V(log.LogLevelExtended).InfoS("finished wave", "type", in.wave.waveType, "count", in.wave.Len(), "duration", time.Since(now))
	return components, errors
}

func (in *WaveProcessor) runWorkers(ctx context.Context, onWorkerDone func()) {
	for i := 0; i < in.concurrentApplies; i++ {
		go func() {
			defer onWorkerDone()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if !in.processNextWorkItem(ctx) {
						klog.V(log.LogLevelTrace).InfoS("queue drained, exiting", "worker", i)
						return
					}

					// Sleep only when there is at least one full batch waiting and we are at max concurrency.
					// This avoids delay when the remaining items are fewer than the number of workers.
					if in.maxConcurrentApplies == in.concurrentApplies && in.queue.Len() > in.concurrentApplies {
						time.Sleep(common.WithJitter(in.deQueueDelay))
					}
				}
			}
		}()
	}
}

func (in *WaveProcessor) processNextWorkItem(ctx context.Context) bool {
	id, shutdown := in.queue.Get()
	if shutdown {
		return false
	}

	defer in.queue.Done(id)

	resource, exists := in.keyToResource[id]
	if !exists {
		klog.V(log.LogLevelTrace).InfoS("resource not found in keyToResource map, continuing", "id", id)
		return true
	}

	in.processWaveItem(ctx, id, resource)
	return true
}

func (in *WaveProcessor) processWaveItem(ctx context.Context, id smcommon.Key, resource unstructured.Unstructured) {
	now := time.Now()

	switch in.wave.waveType {
	case DeleteWave:
		klog.V(log.LogLevelDebug).InfoS("deleting resource", "resource", id)
		in.onDelete(ctx, resource)
	case ApplyWave:
		klog.V(log.LogLevelDebug).InfoS("applying resource", "resource", id)
		in.onApply(ctx, resource)
	case WaitCRDWave:
		klog.V(log.LogLevelDebug).InfoS("waiting for CRD", "resource", id)
		in.onWaitCRD(ctx, resource)
	}

	klog.V(log.LogLevelDebug).InfoS("finished processing wave item", "resource", id, "duration", time.Since(now))
}

func (in *WaveProcessor) onWaitCRD(_ context.Context, resource unstructured.Unstructured) (components []console.ComponentAttributes, errors []console.ServiceErrorAttributes) {
	_ = wait.ExponentialBackoff(wait.Backoff{Duration: 50 * time.Millisecond, Jitter: 3, Steps: 3, Cap: 500 * time.Millisecond}, func() (bool, error) {
		group, version, err := extractGroupAndVersion(resource)
		if err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to extract group and version", "resource", resource.GetUID())
			return true, nil
		}

		_, err = in.discoveryClient.ServerResourcesForGroupVersion(fmt.Sprintf("%s/%s", group, version))
		if err == nil {
			return true, nil
		}
		klog.V(log.LogLevelDebug).Info("waiting for CRD to be established", "group", group, "version", version, "error", err.Error())
		return false, nil
	})
	return
}

func extractGroupAndVersion(u unstructured.Unstructured) (string, string, error) {
	group, found, err := unstructured.NestedString(u.Object, "spec", "group")
	if err != nil || !found {
		return "", "", fmt.Errorf("group not found: %v", err)
	}

	versions, found, err := unstructured.NestedSlice(u.Object, "spec", "versions")
	if err != nil || !found {
		return "", "", fmt.Errorf("versions not found: %v", err)
	}

	var servedVersion string
	for _, v := range versions {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(vm, "name")
		served, _, _ := unstructured.NestedBool(vm, "served")
		if served {
			servedVersion = name
			break
		}
	}

	if servedVersion == "" {
		return "", "", fmt.Errorf("no served version found")
	}

	return group, servedVersion, nil
}

func (in *WaveProcessor) onDelete(ctx context.Context, resource unstructured.Unstructured) {
	live, err := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if err := streamline.GetGlobalStore().DeleteComponent(smcommon.NewStoreKeyFromUnstructured(resource)); err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component from store", "resource", resource.GetUID())
		}

		return
	}

	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to get resource from store", "resource", resource.GetUID())
		return
	}

	if live.GetAnnotations() != nil && live.GetAnnotations()[smcommon.LifecycleDeleteAnnotation] == smcommon.PreventDeletion {
		if err := streamline.GetGlobalStore().DeleteComponent(smcommon.NewStoreKeyFromUnstructured(lo.FromPtr(live))); err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component", "resource", live.GetUID())
		}

		// skip deletion when prevented by annotation
		return
	}

	c := in.client.Resource(helpers.GVRFromGVK(live.GroupVersionKind())).Namespace(live.GetNamespace())
	err = c.Delete(ctx, live.GetName(), metav1.DeleteOptions{
		DryRun: lo.Ternary(in.dryRun, []string{metav1.DryRunAll}, []string{}),
	})
	if errors.IgnoreNotFound(err) != nil {
		in.errorsChan <- console.ServiceErrorAttributes{
			Source:  "delete",
			Message: fmt.Sprintf("failed to delete %s/%s: %s", live.GetNamespace(), live.GetName(), err.Error()),
		}
		return
	}

	if in.dryRun {
		component := common.ToComponentAttributes(live)
		component = in.withDryRun(ctx, component, lo.FromPtr(live), true)
		in.componentChan <- lo.FromPtr(component)

		return
	}

	if err := streamline.GetGlobalStore().DeleteComponent(smcommon.NewStoreKeyFromUnstructured(lo.FromPtr(live))); err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component", "resource", live.GetUID())
	}
}

func (in *WaveProcessor) onApply(ctx context.Context, resource unstructured.Unstructured) {
	entry, _ := streamline.GetGlobalStore().GetComponent(resource)
	if managed := in.isManaged(entry, resource); managed {
		in.errorsChan <- console.ServiceErrorAttributes{
			Source:  "apply",
			Message: fmt.Sprintf("resource %s/%s is already managed by another service %s", resource.GetKind(), resource.GetName(), entry.ServiceID),
			Warning: lo.ToPtr(true),
		}
		resource.SetUID(types.UID(entry.UID))
		in.componentChan <- lo.FromPtr(common.ToComponentAttributes(&resource))
		return
	}

	c := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace())
	appliedResource, err := c.Apply(ctx, resource.GetName(), &resource, metav1.ApplyOptions{
		FieldManager: smcommon.ClientFieldManager,
		Force:        true,
		DryRun:       lo.Ternary(in.dryRun, []string{metav1.DryRunAll}, []string{}),
	})

	if err != nil {
		if err := streamline.GetGlobalStore().ExpireSHA(resource); err != nil {
			klog.ErrorS(err, "failed to expire sha", "resource", resource.GetName())
		}

		in.errorsChan <- console.ServiceErrorAttributes{
			Source:  "apply",
			Message: fmt.Sprintf("failed to apply %s/%s: %s", resource.GetNamespace(), resource.GetName(), err.Error()),
		}

		return
	}

	if in.onApplyCallback != nil {
		in.onApplyCallback(resource)
	}

	if in.dryRun {
		component := common.ToComponentAttributes(&resource)
		component = in.withDryRun(ctx, component, lo.FromPtr(appliedResource), false)
		in.componentChan <- lo.FromPtr(component)

		return
	}

	if err := streamline.GetGlobalStore().UpdateComponentSHA(lo.FromPtr(appliedResource), store.ApplySHA); err != nil {
		klog.V(log.LogLevelExtended).ErrorS(err, "failed to update component SHA", "resource", resource.GetName())
	}
	if err := streamline.GetGlobalStore().UpdateComponentSHA(lo.FromPtr(appliedResource), store.ServerSHA); err != nil {
		klog.V(log.LogLevelExtended).ErrorS(err, "failed to update component SHA", "resource", resource.GetName())
	}
	if err := streamline.GetGlobalStore().CommitTransientSHA(lo.FromPtr(appliedResource)); err != nil {
		klog.V(log.LogLevelExtended).ErrorS(err, "failed to commit transient SHA", "resource", resource.GetName())
	}

	in.componentChan <- lo.FromPtr(common.ToComponentAttributes(appliedResource))
}

func (in *WaveProcessor) isManaged(entry *smcommon.Entry, resource unstructured.Unstructured) bool {
	if entry == nil {
		return false
	}

	_, err := in.svcCache.Get(entry.ServiceID)
	if errors.IsNotFound(err) {
		return false
	}

	serviceID := smcommon.GetOwningInventory(resource)
	return len(entry.ServiceID) > 0 && len(serviceID) > 0 && entry.ServiceID != serviceID
}

func (in *WaveProcessor) withDryRun(ctx context.Context, component *console.ComponentAttributes, resource unstructured.Unstructured, delete bool) *console.ComponentAttributes {
	desiredJSON := asJSON(&resource)
	if delete {
		desiredJSON = "# n/a"
	}

	liveJSON := "# n/a"
	liveResource := in.refetch(ctx, resource)
	if liveResource != nil {
		liveJSON = asJSON(liveResource)
	}

	component.Synced = liveJSON == desiredJSON
	component.Content = &console.ComponentContentAttributes{
		Desired: &desiredJSON,
		Live:    &liveJSON,
	}
	component.State = common.ToStatus(&resource)
	component.Version = resource.GroupVersionKind().Version

	return component
}

func (in *WaveProcessor) refetch(ctx context.Context, resource unstructured.Unstructured) *unstructured.Unstructured {
	result, err := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil
	}

	return result
}

func (in *WaveProcessor) init() {
	in.concurrentApplies = in.maxConcurrentApplies

	if len(in.wave.items) < in.maxConcurrentApplies {
		klog.V(log.LogLevelDebug).InfoS("optimizing concurrent applies", "max", in.maxConcurrentApplies, "optimized", in.wave.Len())
		in.concurrentApplies = len(in.wave.items)
	}

	in.componentChan = make(chan console.ComponentAttributes, in.concurrentApplies)
	in.errorsChan = make(chan console.ServiceErrorAttributes, in.concurrentApplies)
	in.keyToResource = make(map[smcommon.Key]unstructured.Unstructured)
	in.queue = workqueue.NewTyped[smcommon.Key]()

	for _, obj := range in.wave.items {
		key := smcommon.NewKeyFromUnstructured(obj)
		in.keyToResource[key] = obj
		in.queue.Add(key)
	}
}

type WaveProcessorOption func(*WaveProcessor)

// TODO: export to args
func WithWaveMaxConcurrentApplies(n int) WaveProcessorOption {
	return func(w *WaveProcessor) {
		if n < 1 {
			n = defaultMaxConcurrentApplies
		}
		w.maxConcurrentApplies = n
	}
}

// TODO: export to args
func WithWaveDeQueueDelay(d time.Duration) WaveProcessorOption {
	return func(w *WaveProcessor) {
		if d <= 0 {
			d = defaultDeQueueDelay
		}
		w.deQueueDelay = d
	}
}

func WithWaveDryRun(dryRun bool) WaveProcessorOption {
	return func(w *WaveProcessor) {
		w.dryRun = dryRun
	}
}

func WithWaveOnApply(onApply func(resource unstructured.Unstructured)) WaveProcessorOption {
	return func(w *WaveProcessor) {
		w.onApplyCallback = onApply
	}
}

func WithSvcCache(c *consoleclient.Cache[console.ServiceDeploymentForAgent]) WaveProcessorOption {
	return func(w *WaveProcessor) {
		w.svcCache = c
	}
}

func NewWaveProcessor(dynamicClient dynamic.Interface, discoveryClient discovery.DiscoveryInterface, wave Wave, opts ...WaveProcessorOption) *WaveProcessor {
	result := &WaveProcessor{
		mu:                   sync.Mutex{},
		client:               dynamicClient,
		discoveryClient:      discoveryClient,
		wave:                 wave,
		maxConcurrentApplies: defaultMaxConcurrentApplies,
		deQueueDelay:         defaultDeQueueDelay,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}
