package sync

import (
	"github.com/pluralsh/deployment-operator/pkg/applier"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"time"

	generated "github.com/pluralsh/deployment-operator/generated/client/clientset/versioned"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/inventory"
)

type Engine struct {
	client            *client.Client
	clientset         *kubernetes.Clientset
	genClientset      *generated.Clientset
	ctrlClient        ctrlclient.Client
	svcQueue          workqueue.RateLimitingInterface
	deathChan         chan interface{}
	svcCache          *client.ServiceCache
	manifestCache     *manifests.ManifestCache
	syncing           string
	invFactory        inventory.ClientFactory
	applier           *applier.Applier
	destroyer         *apply.Destroyer
	utilFactory       util.Factory
	processingTimeout time.Duration
	gateQueue         workqueue.RateLimitingInterface
}

func New(
	utilFactory util.Factory,
	invFactory inventory.ClientFactory,
	applier *applier.Applier,
	destroyer *apply.Destroyer,
	client *client.Client,
	ctrlclient ctrlclient.Client,
	genClientset *generated.Clientset,
	svcQueue workqueue.RateLimitingInterface,
	svcCache *client.ServiceCache,
	manCache *manifests.ManifestCache,
	processingTimeout time.Duration,
	gateQueue workqueue.RateLimitingInterface,
) *Engine {
	return &Engine{
		client:            client,
		ctrlClient:        ctrlclient,
		genClientset:      genClientset,
		svcQueue:          svcQueue,
		svcCache:          svcCache,
		manifestCache:     manCache,
		invFactory:        invFactory,
		applier:           applier,
		destroyer:         destroyer,
		utilFactory:       utilFactory,
		processingTimeout: processingTimeout,
		gateQueue:         gateQueue,
	}
}

func (engine *Engine) AddHealthCheck(health chan interface{}) {
	engine.deathChan = health
}

func (engine *Engine) WithConfig(config *rest.Config) error {
	cs, err := kubernetes.NewForConfig(config)
	engine.clientset = cs
	return err
}

func (engine *Engine) RegisterHandlers() {}

func (engine *Engine) WipeCache() {
	engine.svcCache.Wipe()
	engine.manifestCache.Wipe()
}
