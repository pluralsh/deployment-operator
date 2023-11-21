package agent

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/klogr"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apply"
	"sigs.k8s.io/cli-utils/pkg/inventory"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	deploysync "github.com/pluralsh/deployment-operator/pkg/sync"
	"github.com/pluralsh/deployment-operator/pkg/websocket"
)

var (
	log = klogr.New()
)

type Agent struct {
	consoleClient   *client.Client
	discoveryClient *discovery.DiscoveryClient
	config          *rest.Config
	engine          *deploysync.Engine
	deathChan       chan interface{}
	svcQueue        workqueue.RateLimitingInterface
	socket          *websocket.Socket
	refresh         time.Duration
}

func New(config *rest.Config, refresh, processingTimeout time.Duration, consoleUrl, deployToken, clusterId string) (*Agent, error) {
	disableClientLimits(config)
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	consoleClient := client.New(consoleUrl, deployToken)
	svcCache := client.NewCache(consoleClient, refresh)
	manifestCache := manifests.NewCache(refresh, deployToken)

	svcQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	deathChan := make(chan interface{})
	invFactory := inventory.ClusterClientFactory{StatusPolicy: inventory.StatusPolicyNone}

	socket, err := websocket.New(clusterId, consoleUrl, deployToken, svcQueue, svcCache)
	if err != nil {
		if socket == nil {
			return nil, err
		}
		log.Error(err, "could not initiate websocket connection, ignoring and falling back to polling")
	}

	f := newFactory(config)

	applier, err := newApplier(invFactory, f)
	if err != nil {
		return nil, err
	}
	destroyer, err := newDestroyer(invFactory, f)
	if err != nil {
		return nil, err
	}
	engine := deploysync.New(f, invFactory, applier, destroyer, consoleClient, svcQueue, svcCache, manifestCache, processingTimeout)
	engine.AddHealthCheck(deathChan)
	if err := engine.WithConfig(config); err != nil {
		return nil, err
	}

	return &Agent{
		discoveryClient: dc,
		consoleClient:   consoleClient,
		engine:          engine,
		deathChan:       deathChan,
		svcQueue:        svcQueue,
		socket:          socket,
		refresh:         refresh,
	}, nil
}

func (agent *Agent) Run() {
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
		if err := agent.socket.Join(); err != nil {
			log.Error(err, "could not establish websocket to upstream")
		}

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
		vs := strings.Split(info.GitVersion, "-")
		if err := agent.consoleClient.Ping(strings.TrimPrefix(vs[0], "v")); err != nil {
			log.Error(err, "failed to ping cluster after scheduling syncs")
		}
		agent.engine.ScrapeKube()
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

func newFactory(cfg *rest.Config) util.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag()
	kubeConfigFlags.WithDiscoveryQPS(cfg.QPS).WithDiscoveryBurst(cfg.Burst)
	cfgPtrCopy := cfg
	kubeConfigFlags.WrapConfigFn = func(c *rest.Config) *rest.Config {
		// update rest.Config to pick up QPS & timeout changes
		deepCopyRESTConfig(cfgPtrCopy, c)
		return c
	}
	matchVersionKubeConfigFlags := util.NewMatchVersionFlags(kubeConfigFlags)
	return util.NewFactory(matchVersionKubeConfigFlags)
}

func newApplier(invFactory inventory.ClientFactory, f util.Factory) (*apply.Applier, error) {
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return nil, err
	}

	return apply.NewApplierBuilder().
		WithFactory(f).
		WithInventoryClient(invClient).
		Build()
}

func newDestroyer(invFactory inventory.ClientFactory, f util.Factory) (*apply.Destroyer, error) {
	invClient, err := invFactory.NewClient(f)
	if err != nil {
		return nil, err
	}

	return apply.NewDestroyerBuilder().
		WithFactory(f).
		WithInventoryClient(invClient).
		Build()
}

func deepCopyRESTConfig(from, to *rest.Config) {
	to.Host = from.Host
	to.APIPath = from.APIPath
	to.ContentConfig = from.ContentConfig
	to.Username = from.Username
	to.Password = from.Password
	to.BearerToken = from.BearerToken
	to.BearerTokenFile = from.BearerTokenFile
	to.Impersonate = rest.ImpersonationConfig{
		UserName: from.Impersonate.UserName,
		UID:      from.Impersonate.UID,
		Groups:   from.Impersonate.Groups,
		Extra:    from.Impersonate.Extra,
	}
	to.AuthProvider = from.AuthProvider
	to.AuthConfigPersister = from.AuthConfigPersister
	to.ExecProvider = from.ExecProvider
	if from.ExecProvider != nil && from.ExecProvider.Config != nil {
		to.ExecProvider.Config = from.ExecProvider.Config.DeepCopyObject()
	}
	to.TLSClientConfig = rest.TLSClientConfig{
		Insecure:   from.TLSClientConfig.Insecure,
		ServerName: from.TLSClientConfig.ServerName,
		CertFile:   from.TLSClientConfig.CertFile,
		KeyFile:    from.TLSClientConfig.KeyFile,
		CAFile:     from.TLSClientConfig.CAFile,
		CertData:   from.TLSClientConfig.CertData,
		KeyData:    from.TLSClientConfig.KeyData,
		CAData:     from.TLSClientConfig.CAData,
		NextProtos: from.TLSClientConfig.NextProtos,
	}
	to.UserAgent = from.UserAgent
	to.DisableCompression = from.DisableCompression
	to.Transport = from.Transport
	to.WrapTransport = from.WrapTransport
	to.QPS = from.QPS
	to.Burst = from.Burst
	to.RateLimiter = from.RateLimiter
	to.WarningHandler = from.WarningHandler
	to.Timeout = from.Timeout
	to.Dial = from.Dial
	to.Proxy = from.Proxy
}
