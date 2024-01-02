package ping

import (
	"context"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Pinger struct {
	consoleClient   *client.Client
	discoveryClient *discovery.DiscoveryClient
	factory         util.Factory
}

func New(console *client.Client, discovery *discovery.DiscoveryClient, factory util.Factory) *Pinger {
	return &Pinger{
		consoleClient:   console,
		discoveryClient: discovery,
		factory:         factory,
	}
}

func (p *Pinger) Ping() error {
	info, err := p.discoveryClient.ServerVersion()
	if err != nil {
		return err
	}

	cs, err := p.factory.KubernetesClientSet()
	if err != nil {
		return nil
	}

	podNames := []string{}
	// can find some distro information by checking what's running in kube-system
	if pods, err := cs.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{}); err == nil {
		podNames = lo.Map(pods.Items, func(pod corev1.Pod, ind int) string {
			return pod.Name
		})
	}

	attrs := pingAttributes(info, podNames)
	if err := p.consoleClient.PingCluster(attrs); err != nil {
		attrs.Distro = nil
		return p.consoleClient.PingCluster(attrs) // fallback to no distro to support old console servers
	}

	return nil
}
