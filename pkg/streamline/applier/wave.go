package applier

import (
	"context"
	"github.com/pluralsh/deployment-operator/pkg/streamline"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
	"k8s.io/klog/v2"
	"math/rand"
	"sync"
	"time"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

type WaveType string

const (
	ApplyWave  WaveType = "apply"
	DeleteWave WaveType = "delete"
)

type Wave struct {
	unstructured.UnstructuredList

	Type WaveType
	// TODO: handle that
	DryRun bool
}

func NewWave(resources []unstructured.Unstructured, waveType WaveType) Wave {
	return Wave{
		UnstructuredList: unstructured.UnstructuredList{Items: resources},
		Type:             waveType,
	}
}

func (in *Wave) Add(resource unstructured.Unstructured) {
	in.Items = append(in.Items, resource)
}

type Waves []Wave

func NewWaves(resources unstructured.UnstructuredList) Waves {
	defaultWaveCount := 5
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

	for _, resource := range resources.Items {
		if waveIdx, exists := kindToWave[resource.GetKind()]; exists {
			waves[waveIdx].Add(resource)
		} else {
			// Unknown resource kind, put it in the last wave (4)
			waves[4].Add(resource)
		}
	}

	return waves
}

type WaveProcessor struct {
	mu sync.Mutex

	client        dynamic.Interface
	wave          Wave
	componentChan chan client.ComponentAttributes
	errorsChan    chan client.ServiceErrorAttributes
	queue         *workqueue.Typed[Key]
	keyToResource map[Key]unstructured.Unstructured

	MaxConcurrentApplies int
	DeQueueJitter        time.Duration
}

func (in *WaveProcessor) Run(ctx context.Context) (components []client.ComponentAttributes, errors []client.ServiceErrorAttributes) {
	in.mu.Lock()

	defer close(in.componentChan)
	defer close(in.errorsChan)

	for _, obj := range in.wave.Items {
		key := NewKeyFromUnstructured(obj)
		in.keyToResource[key] = obj
		in.queue.Add(key)
	}

	wg := &sync.WaitGroup{}

	func() {
		defer in.mu.Unlock()

		wg.Add(in.MaxConcurrentApplies)
		for i := 0; i < in.MaxConcurrentApplies; i++ {
			go func() {
				defer wg.Done()
				for in.processNextWorkItem(ctx) {
					time.Sleep(time.Duration(rand.Int63n(int64(in.DeQueueJitter))))
				}
			}()
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case component, ok := <-in.componentChan:
				if !ok {
					return
				}

				components = append(components, component)
			case err, ok := <-in.errorsChan:
				if !ok {
					return
				}

				errors = append(errors, err)
			}
		}
	}()

	wg.Wait()

	return components, errors
}

func (in *WaveProcessor) processNextWorkItem(ctx context.Context) bool {
	if in.queue.Len() == 0 {
		in.queue.ShutDown()
		return false
	}
	id, shutdown := in.queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	defer in.queue.Done(id)
	resource, exists := in.keyToResource[id]
	if !exists {
		return false
	}

	c := in.client.Resource(helpers.GVRFromGVK(resource.GroupVersionKind())).Namespace(resource.GetNamespace())
	switch in.wave.Type {
	case DeleteWave:
		err := c.Delete(ctx, resource.GetName(), metav1.DeleteOptions{})
		if err != nil {
			in.errorsChan <- client.ServiceErrorAttributes{
				Source:  "sync",
				Message: err.Error(),
			}
		}
	case ApplyWave:
		appliedResource, err := c.Apply(ctx, resource.GetName(), &resource, metav1.ApplyOptions{
			FieldManager: "plural-sync",
		})
		if err != nil {
			if err := streamline.GlobalStore().ExpireSHA(resource); err != nil {
				klog.ErrorS(err, "failed to expire sha", "resource", resource.GetName())
			}
			in.errorsChan <- client.ServiceErrorAttributes{
				Source:  "sync",
				Message: err.Error(),
			}
			break
		}
		if err := streamline.GlobalStore().UpdateComponentSHA(lo.FromPtr(appliedResource), store.ApplySHA); err != nil {
			klog.Errorf("Failed to update component SHA: %v", err)
		}
		if err := streamline.GlobalStore().CommitTransientSHA(lo.FromPtr(appliedResource)); err != nil {
			klog.Errorf("Failed to commit transient SHA: %v", err)
		}

		in.componentChan <- lo.FromPtr(common.ToComponentAttributes(appliedResource))
	}

	return true
}

func NewWaveProcessor(dynamicClient dynamic.Interface, wave Wave) *WaveProcessor {
	return &WaveProcessor{
		mu:                   sync.Mutex{},
		client:               dynamicClient,
		wave:                 wave,
		componentChan:        make(chan client.ComponentAttributes),
		errorsChan:           make(chan client.ServiceErrorAttributes),
		queue:                workqueue.NewTyped[Key](),
		keyToResource:        make(map[Key]unstructured.Unstructured),
		MaxConcurrentApplies: 10,
		DeQueueJitter:        time.Second,
	}
}
