package sync

import (
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/pluralsh/deployment-operator/agent/pkg/client"
	"github.com/pluralsh/deployment-operator/agent/pkg/manifests"
)

type Engine struct {
	client        *client.Client
	svcChan       chan string
	deathChan     chan interface{}
	svcCache      *client.ServiceCache
	manifestCache *manifests.ManifestCache
	engine        engine.GitOpsEngine
	unsubscribe   cache.Unsubscribe
	cache         cache.ClusterCache
	syncing       string
}

func New(engine engine.GitOpsEngine, cache cache.ClusterCache, client *client.Client, svcChan chan string, svcCache *client.ServiceCache, manCache *manifests.ManifestCache) *Engine {
	return &Engine{
		client:        client,
		cache:         cache,
		engine:        engine,
		svcChan:       svcChan,
		svcCache:      svcCache,
		manifestCache: manCache,
	}
}

func (engine *Engine) AddHealthCheck(health chan interface{}) {
	engine.deathChan = health
}

func (engine *Engine) RegisterHandlers() {
	engine.unsubscribe = engine.cache.OnResourceUpdated(func(new *cache.Resource, old *cache.Resource, nrs map[kube.ResourceKey]*cache.Resource) {
		syncing := engine.syncing
		if id := svcId(new); id != nil && *id != syncing {
			engine.svcChan <- *id
		} else if id := svcId(old); id != nil && *id != syncing {
			engine.svcChan <- *id
		}
	})
}
