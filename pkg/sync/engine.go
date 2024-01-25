package sync

import (
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

type Engine struct {
	Client            *client.Client
	Clientset         *kubernetes.Clientset
	svcQueue          workqueue.RateLimitingInterface
	deathChan         chan interface{}
	SvcCache          *client.ServiceCache
	ManifestCache     *manifests.ManifestCache
	syncing           string
	invFactory        inventory.ClientFactory
	Applier           *applier.Applier
	Destroyer         *apply.Destroyer
	UtilFactory       util.Factory
	processingTimeout time.Duration
	LuaScript         string
}

func New(utilFactory util.Factory, invFactory inventory.ClientFactory, applier *applier.Applier, destroyer *apply.Destroyer, client *client.Client, svcQueue workqueue.RateLimitingInterface, svcCache *client.ServiceCache, manCache *manifests.ManifestCache, processingTimeout time.Duration) *Engine {
	return &Engine{
		Client:            client,
		svcQueue:          svcQueue,
		SvcCache:          svcCache,
		ManifestCache:     manCache,
		invFactory:        invFactory,
		Applier:           applier,
		Destroyer:         destroyer,
		UtilFactory:       utilFactory,
		processingTimeout: processingTimeout,
	}
}

func (engine *Engine) AddHealthCheck(health chan interface{}) {
	engine.deathChan = health
}

func (engine *Engine) WithConfig(config *rest.Config) error {
	cs, err := kubernetes.NewForConfig(config)
	engine.Clientset = cs
	return err
}

func (engine *Engine) WipeCache() {
	engine.SvcCache.Wipe()
	engine.ManifestCache.Wipe()
}

func (engine *Engine) GetLuaScript() string {
	return engine.LuaScript
}

func (engine *Engine) SetLuaScript(script string) {
	engine.LuaScript = script
}
