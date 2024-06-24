package cache

import (
	"context"
	"slices"
	"strings"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling/event"
	"sigs.k8s.io/cli-utils/pkg/kstatus/status"
)

var healthCache *HealthCache

type HealthCache struct {
	ctx           context.Context
	consoleClient client.Client
	cache         cmap.ConcurrentMap[string, Health]
}

func (in *HealthCache) Update(e *event.ResourceStatus) {
	if e == nil || e.Resource == nil {
		return
	}

	serviceID := UnstructuredServiceDeploymentToID(e.Resource)
	health, _ := in.cache.Get(serviceID)
	health.AddHealthState(e)
	in.cache.Set(serviceID, health)
}

func (in *HealthCache) startPoller() {
	go wait.PollUntilContextCancel(in.ctx, 1*time.Minute, false, func(_ context.Context) (done bool, err error) {
		in.reconcile()
		return false, nil
	})
}

func (in *HealthCache) reconcile() {
	for serviceID, health := range in.cache.Items() {
		currentSha := health.CalculateSHA()
		if health.GetSHA() == currentSha {
			continue
		}

		if err := in.consoleClient.UpdateComponents(serviceID, in.toComponentAttributes(health.Items()), nil); err != nil {
			log.Logger.Errorw("error during updating service components", "service_id", serviceID, "error", err)
			return
		}

		health.SetSHA(currentSha)
		in.cache.Set(serviceID, health)
	}
}

func (in *HealthCache) toComponentAttributes(healthComponents []lo.Entry[string, HealthComponent]) []*console.ComponentAttributes {
	return algorithms.Map(healthComponents, func(entry lo.Entry[string, HealthComponent]) *console.ComponentAttributes {
		key, _ := ParseResourceKey(entry.Key)
		return &console.ComponentAttributes{
			State:     entry.Value.state,
			Synced:    entry.Value.status == status.CurrentStatus,
			Group:     key.GroupKind.Group,
			Version:   entry.Value.version,
			Kind:      key.GroupKind.Kind,
			Namespace: key.Namespace,
			Name:      key.Name,
		}
	})
}

func InitHealthCache(consoleClient client.Client) {
	healthCache = &HealthCache{
		ctx:           context.Background(),
		consoleClient: consoleClient,
		cache:         cmap.New[Health](),
	}

	healthCache.startPoller()
}

func GetHealthCache() *HealthCache {
	if healthCache == nil {
		log.Logger.Fatalln("healthCache not initialized. Make sure to initialize it with InitHealthCache()")
	}

	return healthCache
}

type HealthComponent struct {
	state   *console.ComponentState
	status  status.Status
	version string
}

func (in HealthComponent) String() string {
	if in.state == nil {
		return ""
	}

	return string(*in.state)
}

type Health struct {
	// sha is the last SHA calculated based on the cache items.
	sha string

	// cache maps a resource key to a service component state
	cache map[string]HealthComponent
}

func (in *Health) Items() []lo.Entry[string, HealthComponent] {
	return lo.ToPairs(in.cache)
}

func (in *Health) AddHealthState(e *event.ResourceStatus) {
	if e == nil || e.Resource == nil {
		return
	}

	if in.cache == nil {
		in.cache = map[string]HealthComponent{}
	}

	in.cache[e.Identifier.String()] = HealthComponent{
		state:   getResourceHealth(e.Resource),
		version: e.Resource.GetAPIVersion(),
		status:  e.Status,
	}
}

func (in *Health) RemoveHealthState(key string) {
	delete(in.cache, key)
}

func (in *Health) SetSHA(sha string) {
	in.sha = sha
}

func (in *Health) GetSHA() string {
	return in.sha
}

func (in *Health) CalculateSHA() string {
	builder := strings.Builder{}
	keys := lo.Keys(in.cache)
	slices.Sort(keys)

	for _, key := range keys {
		val := in.cache[key]
		builder.WriteString(key)
		builder.WriteString(val.String())
	}

	return utils.HashString(builder.String())
}

// getResourceHealth returns the health of a resource.
func getResourceHealth(obj *unstructured.Unstructured) *console.ComponentState {
	if obj.GetDeletionTimestamp() != nil {
		return lo.ToPtr(console.ComponentStatePending)
	}

	healthCheckFunc := common.GetHealthCheckFuncByGroupVersionKind(obj.GroupVersionKind())
	if healthCheckFunc == nil {
		return lo.ToPtr(console.ComponentStatePending)
	}

	health, err := healthCheckFunc(obj)
	if err != nil {
		return nil
	}

	switch health.Status {
	case common.HealthStatusDegraded:
		return lo.ToPtr(console.ComponentStateFailed)
	case common.HealthStatusHealthy:
		return lo.ToPtr(console.ComponentStateRunning)
	case common.HealthStatusPaused:
		return lo.ToPtr(console.ComponentStatePaused)
	}

	return lo.ToPtr(console.ComponentStatePending)
}
