package ping

import (
	"fmt"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/util"
	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pluralsh/deployment-operator/internal/utils"
	discoverycache "github.com/pluralsh/deployment-operator/pkg/cache/discovery"
	"github.com/pluralsh/deployment-operator/pkg/client"
)

type Pinger struct {
	consoleClient  client.Client
	discoveryCache discoverycache.Cache
	factory        util.Factory
	k8sClient      ctrclient.Client
	clientset      *kubernetes.Clientset
	apiExtClient   *apiextensionsclient.Clientset
}

func NewOrDie(console client.Client, config *rest.Config, k8sClient ctrclient.Client, discoveryCache discoverycache.Cache) *Pinger {
	pinger, err := New(console, config, k8sClient, discoveryCache)
	if err != nil {
		panic(fmt.Errorf("failed to create Pinger: %w", err))
	}
	return pinger
}

func New(console client.Client, config *rest.Config, k8sClient ctrclient.Client, discoveryCache discoverycache.Cache) (*Pinger, error) {
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
		consoleClient:  console,
		factory:        f,
		k8sClient:      k8sClient,
		clientset:      cs,
		apiExtClient:   apiExtClient,
		discoveryCache: discoveryCache,
	}, nil
}
