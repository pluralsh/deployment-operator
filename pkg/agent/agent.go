package agent

import (
	"fmt"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/pkg/sync"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
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
	svcQueue        workqueue.RateLimitingInterface
	cleanup         engine.StopFunc
	refresh         time.Duration
}

func New(config *rest.Config, refresh time.Duration, consoleUrl, deployToken string) (*Agent, error) {
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
		svcQueue:        svcQueue,
		cleanup:         cleanup,
		refresh:         refresh,
	}, nil
}

func (agent *Agent) Run() {
	defer agent.cleanup()
	defer agent.svcQueue.ShutDown()
	defer agent.engine.WipeCache()
	go func() {
		for {
			go agent.engine.ControlLoop()
			failure := <-agent.deathChan
			fmt.Printf("recovered from panic %v\n", failure)
		}
	}()

	err := wait.PollInfinite(agent.refresh, func() (done bool, err error) {
		log.Info("fetching services for cluster")
		svcs, err := agent.consoleClient.GetServices()
		if err != nil {
			log.Error(err, "failed to fetch service list from deployments service")
			return false, nil
		}

		for _, svc := range svcs {
			log.Info("sending update for", "service", svc.ID)
			agent.svcQueue.Add(svc.ID)
		}

		info, err := agent.discoveryClient.ServerVersion()
		if err != nil {
			log.Error(err, "failed to fetch cluster version")
			return false, nil
		}
		v := fmt.Sprintf("%s.%s", info.Major, info.Minor)
		if err := agent.consoleClient.Ping(v); err != nil {
			log.Error(err, "failed to ping cluster after scheduling syncs")
		}
		return false, nil
	})
	if err != nil {
		return
	}
}

// SetupWithManager sets up the controller with the Manager.
func (agent *Agent) SetupWithManager() error {
	go func() {
		agent.Run()
	}()
	return nil
}
