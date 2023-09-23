package agent

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2/klogr"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"

	"github.com/pluralsh/deployment-operator/agent/pkg/client"
	"github.com/pluralsh/deployment-operator/agent/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/agent/pkg/sync"
)

var (
	log = klogr.New()
)

type Agent struct {
	consoleClient *client.Client
	engine        *deploysync.Engine
	deathChan     chan interface{}
	svcChan       chan string
	cleanup       engine.StopFunc
	refresh       time.Duration
}

func New(clientConfig clientcmd.ClientConfig, refresh time.Duration, consoleUrl, deployToken string) (*Agent, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	consoleClient := client.New(consoleUrl, deployToken)
	svcCache := client.NewCache(consoleClient, refresh)
	manifestCache := manifests.NewCache(refresh)

	svcChan := make(chan string)
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

	engine := deploysync.New(gitOpsEngine, clusterCache, consoleClient, svcChan, svcCache, manifestCache)
	engine.RegisterHandlers()
	engine.AddHealthCheck(deathChan)

	return &Agent{
		consoleClient: consoleClient,
		engine:        engine,
		deathChan:     deathChan,
		svcChan:       svcChan,
		cleanup:       cleanup,
		refresh:       refresh,
	}, nil
}

func (agent *Agent) Run() {
	defer agent.cleanup()
	go func() {
		for {
			go agent.engine.ControlLoop()
			failure := <-agent.deathChan
			fmt.Printf("recovered from panic %v\n", failure)
		}
	}()

	for {
		svcs, err := agent.consoleClient.GetServices()
		if err != nil {
			log.Error(err, "failed to fetch service list from deployments service")
			time.Sleep(agent.refresh)
			continue
		}

		for _, svc := range svcs {
			agent.svcChan <- svc.ID
		}

		// TODO: fetch kubernetes version properly
		if err := agent.consoleClient.Ping("1.24"); err != nil {
			log.Error(err, "failed to ping cluster after scheduling syncs")
		}

		time.Sleep(agent.refresh)
	}
}
