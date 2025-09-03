package applier

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/workqueue"

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
	ApplyWave  WaveType = "apply"
	DeleteWave WaveType = "delete"
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
	defaultWaveCount := 5
	waves := make([]Wave, 0, defaultWaveCount)
	for i := 0; i < defaultWaveCount; i++ {
		waves = append(waves, NewWave([]unstructured.Unstructured{}, ApplyWave))
	}

	kindToWave := map[string]int{
		// Wave 0 - core non-namespaced resources
		common.NamespaceKind:                0,
		common.CustomResourceDefinitionKind: 0, // TODO: should it be here or in the last wave?
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

	for _, resource := range resources {
		if waveIdx, exists := kindToWave[resource.GetKind()]; exists {
			waves[waveIdx].Add(resource)
		} else {
			// Unknown resource kind, put it in the last wave
			waves[len(waves)-1].Add(resource)
		}
	}

	return waves
}

const (
	defaultMaxConcurrentApplies = 10
	defaultDeQueueJitter        = 100 * time.Millisecond
)

// WaveProcessor processes a wave of resources. It applies or deletes the resources in the wave.
// It uses a work queue to process the items in the wave concurrently. It uses a channel to communicate
// between the workers and the collector goroutine. The collector goroutine collects the components and errors
// from the channels and returns them to the caller.
type WaveProcessor struct {
	mu sync.Mutex

	// client is the dynamic client used to apply the resources
	client dynamic.Interface

	// wave is the wave to be processed. It contains the resources to be applied or deleted.
	wave Wave

	// componentChan is used to communicate between the workers and the collector goroutine
	// when a resource is successfully applied, the worker sends the component attributes to the channel
	componentChan chan client.ComponentAttributes

	// errorsChan is used to communicate between the workers and the collector goroutine
	// when a resource fails to be applied, the worker sends the error to the channel
	errorsChan chan client.ServiceErrorAttributes

	// queue is the work queue used to process the items in the wave
	queue *workqueue.Typed[Key]

	// keyToResource is a map of the wave items to their keys.
	// It is used to lookup the resource from the key when processing the items in the queue.
	keyToResource map[Key]unstructured.Unstructured

	// maxConcurrentApplies is the maximum number of workers that can be started
	maxConcurrentApplies int

	// concurrentApplies is the number of workers that will be started.
	// It is calculated based on the number of items in the wave and the maxConcurrentApplies option.
	// If the wave contains more items than the maxConcurrentApplies option, the number of workers
	// will be set to the maxConcurrentApplies otherwise it will be set to the number of items in the wave.
	concurrentApplies int

	// deQueueJitter is the amount of time to wait before dequeuing the next item from the queue
	// by the same worker.
	deQueueJitter time.Duration

	// dryRun determines if the wave should be applied in dry run mode
	// meaning that the changes will not be persisted
	dryRun bool
}

func (in *WaveProcessor) Run(ctx context.Context) (components []client.ComponentAttributes, errors []client.ServiceErrorAttributes) {
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
		go func(i int) {
			defer onWorkerDone()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if !in.processNextWorkItem(ctx, i) {
						klog.V(log.LogLevelTrace).InfoS("queue drained, exiting", "worker", i)
						return
					}

					// Sleep only when there is at least one full batch waiting and we are at max concurrency.
					// This avoids jitter when the remaining items are fewer than the number of workers.
					if in.maxConcurrentApplies == in.concurrentApplies && in.queue.Len() > in.concurrentApplies {
						time.Sleep(time.Duration(rand.Int63n(int64(in.deQueueJitter))))
					}
				}
			}
		}(i)
	}
}

func (in *WaveProcessor) processNextWorkItem(ctx context.Context, workerNr int) bool {
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

func (in *WaveProcessor) processWaveItem(ctx context.Context, id Key, resource unstructured.Unstructured) {
	now := time.Now()

	switch in.wave.waveType {
	case DeleteWave:
		klog.V(log.LogLevelDebug).InfoS("deleting resource", "resource", id)
		in.onDelete(ctx, resource)
	case ApplyWave:
		klog.V(log.LogLevelDebug).InfoS("applying resource", "resource", id)
		in.onApply(ctx, resource)
	}

	klog.V(log.LogLevelDebug).InfoS("finished processing wave item", "resource", id, "duration", time.Since(now))
}

func (in *WaveProcessor) onDelete(ctx context.Context, resource unstructured.Unstructured) {
	live, err := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if err := streamline.GetGlobalStore().DeleteComponent(resource.GetUID()); err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component from store", "resource", resource.GetUID())
		}

		return
	}

	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to get resource from store", "resource", resource.GetUID())
		return
	}

	if live.GetAnnotations() != nil && live.GetAnnotations()[smcommon.LifecycleDeleteAnnotation] == smcommon.PreventDeletion {
		if err := streamline.GetGlobalStore().DeleteComponent(live.GetUID()); err != nil {
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
		in.errorsChan <- client.ServiceErrorAttributes{
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

	if err := streamline.GetGlobalStore().DeleteComponent(live.GetUID()); err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to delete component", "resource", live.GetUID())
	}
}

func (in *WaveProcessor) onApply(ctx context.Context, resource unstructured.Unstructured) {
	c := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace())
	appliedResource, err := c.Apply(ctx, resource.GetName(), &resource, metav1.ApplyOptions{
		FieldManager: smcommon.ClientFieldManager,
		DryRun:       lo.Ternary(in.dryRun, []string{metav1.DryRunAll}, []string{}),
	})

	if err != nil {
		if err := streamline.GetGlobalStore().ExpireSHA(resource); err != nil {
			klog.ErrorS(err, "failed to expire sha", "resource", resource.GetName())
		}

		in.errorsChan <- client.ServiceErrorAttributes{
			Source:  "apply",
			Message: fmt.Sprintf("failed to apply %s/%s: %s", resource.GetNamespace(), resource.GetName(), err.Error()),
		}

		return
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

func (in *WaveProcessor) withDryRun(ctx context.Context, component *client.ComponentAttributes, resource unstructured.Unstructured, delete bool) *client.ComponentAttributes {
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
	component.Content = &client.ComponentContentAttributes{
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

	in.componentChan = make(chan client.ComponentAttributes, in.concurrentApplies)
	in.errorsChan = make(chan client.ServiceErrorAttributes, in.concurrentApplies)
	in.keyToResource = make(map[Key]unstructured.Unstructured)
	in.queue = workqueue.NewTyped[Key]()

	for _, obj := range in.wave.items {
		key := NewKeyFromUnstructured(obj)
		in.keyToResource[key] = obj
		in.queue.Add(key)
	}
}

type Option func(*WaveProcessor)

func WithMaxConcurrentApplies(n int) Option {
	return func(w *WaveProcessor) {
		if n < 1 {
			n = defaultMaxConcurrentApplies
		}
		w.maxConcurrentApplies = n
	}
}

func WithDeQueueJitter(d time.Duration) Option {
	return func(w *WaveProcessor) {
		if d <= 0 {
			d = defaultDeQueueJitter
		}
		w.deQueueJitter = d
	}
}

func WithDryRun(dryRun bool) Option {
	return func(w *WaveProcessor) {
		w.dryRun = dryRun
	}
}

func NewWaveProcessor(dynamicClient dynamic.Interface, wave Wave, opts ...Option) *WaveProcessor {
	result := &WaveProcessor{
		mu:                   sync.Mutex{},
		client:               dynamicClient,
		wave:                 wave,
		maxConcurrentApplies: defaultMaxConcurrentApplies,
		deQueueJitter:        defaultDeQueueJitter,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}
