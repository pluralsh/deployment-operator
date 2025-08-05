package ping

import (
	"fmt"

	"github.com/pluralsh/deployment-operator/internal/utils"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/client-go/discovery"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Pinger struct {
	consoleClient   client.Client
	discoveryClient *discovery.DiscoveryClient
	factory         util.Factory
	k8sClient       ctrclient.Client
	clientset       *kubernetes.Clientset
	apiExtClient    *apiextensionsclient.Clientset
}

func NewOrDie(console client.Client, config *rest.Config, k8sClient ctrclient.Client) *Pinger {
	pinger, err := New(console, config, k8sClient)
	if err != nil {
		panic(fmt.Errorf("failed to create Pinger: %w", err))
	}
	return pinger
}

func New(console client.Client, config *rest.Config, k8sClient ctrclient.Client) (*Pinger, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}
	f := utils.NewFactory(config)
	cs, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	apiExtClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Pinger{
		consoleClient:   console,
		discoveryClient: discoveryClient,
		factory:         f,
		k8sClient:       k8sClient,
		clientset:       cs,
		apiExtClient:    apiExtClient,
	}, nil
}
