package sync

import (
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"k8s.io/client-go/util/workqueue"
)

type Engine struct {
	client        *client.Client
	svcQueue      workqueue.RateLimitingInterface
	deathChan     chan interface{}
	svcCache      *client.ServiceCache
	manifestCache *manifests.ManifestCache
	engine        engine.GitOpsEngine
	unsubscribe   cache.Unsubscribe
	cache         cache.ClusterCache
	syncing       string
}

func New(engine engine.GitOpsEngine, cache cache.ClusterCache, client *client.Client, svcQueue workqueue.RateLimitingInterface, svcCache *client.ServiceCache, manCache *manifests.ManifestCache) *Engine {
	return &Engine{
		client:        client,
		cache:         cache,
		engine:        engine,
		svcQueue:      svcQueue,
		svcCache:      svcCache,
		manifestCache: manCache,
	}
}

func (engine *Engine) AddHealthCheck(health chan interface{}) {
	engine.deathChan = health
}

func (engine *Engine) RegisterHandlers() {
	engine.unsubscribe = engine.cache.OnResourceUpdated(func(new *cache.Resource, old *cache.Resource, nrs map[kube.ResourceKey]*cache.Resource) {
		if id := svcId(new); id != nil && isRoot(new) {
			engine.svcQueue.Add(*id)
		} else if id := svcId(old); id != nil && isRoot(old) {
			engine.svcQueue.Add(*id)
		}
	})
}

func isRoot(r *cache.Resource) bool {
	return svcId(r) != nil && len(r.OwnerRefs) == 0
}