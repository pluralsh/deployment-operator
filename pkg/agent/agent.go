package agent

import (
	"fmt"
	"time"

	"github.com/alitto/pond"
	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/pkg/sync"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/klogr"
)

var (
	log = klogr.New()
)

type Agent struct {
	consoleClient   *client.Client
	discoveryClient *discovery.DiscoveryClient
	engine          *deploysync.Engine
	deathChan       chan interface{}
	cleanup         engine.StopFunc
	refresh         time.Duration
}

func New(clientConfig clientcmd.ClientConfig, refresh time.Duration, consoleUrl, deployToken string) (*Agent, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	consoleClient := client.New(consoleUrl, deployToken)
	svcCache := client.NewCache(consoleClient, refresh)
	manifestCache := manifests.NewCache(refresh, deployToken)

	svcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	deathChan := make(chan interface{})

	// we should enable SSA if kubernetes version supports it
	clusterCache := cache.NewClusterCache(config,
		cache.SetLogr(log),
		cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
			svcId := un.GetAnnotations()[deploysync.SyncAnnotation]
			sha := un.GetAnnotations()[deploysync.SyncShaAnnotation]
			info = deploysync.NewResource(svcId, sha)
			// cache resources that have the current annotation
			cacheManifest = svcId != ""
			return
		}),
	)

	gitOpsEngine := engine.NewEngine(config, clusterCache, engine.WithLogr(log))
	cleanup, err := gitOpsEngine.Run()
	if err != nil {
		return nil, err
	}

	engine := deploysync.New(gitOpsEngine, clusterCache, consoleClient, svcQueue, svcCache, manifestCache)
	engine.AddHealthCheck(deathChan)

	return &Agent{
		discoveryClient: dc,
		consoleClient:   consoleClient,
		engine:          engine,
		deathChan:       deathChan,
		cleanup:         cleanup,
		refresh:         refresh,
	}, nil
}

func (agent *Agent) Run() {
	defer agent.cleanup()
	defer agent.engine.WipeCache()
	panicHandler := func(p interface{}) {
		fmt.Printf("Task panicked: %v", p)
	}

	for {
		log.Info("fetching services for cluster")
		svcs, err := agent.consoleClient.GetServices()
		if err != nil {
			log.Error(err, "failed to fetch service list from deployments service %v", err)
			time.Sleep(agent.refresh)
			continue
		}
		pool := pond.New(20, 100, pond.MinWorkers(20), pond.PanicHandler(panicHandler))
		for _, svc := range svcs {
			log.Info("sending update for", "service", svc.ID, "namespace", svc.Namespace, "name", svc.Name)
			pool.TrySubmit(func() {
				if err := agent.engine.ProcessItem(svc.ID); err != nil {
					log.Error(err, "found unprocessable error")
				}
			})
		}
		pool.StopAndWait()
		info, err := agent.discoveryClient.ServerVersion()
		if err != nil {
			log.Error(err, "failed to fetch cluster version")
		}
		v := fmt.Sprintf("%s.%s", info.Major, info.Minor)
		if err := agent.consoleClient.Ping(v); err != nil {
			log.Error(err, "failed to ping cluster after scheduling syncs")
		}
		time.Sleep(agent.refresh)
	}
}
