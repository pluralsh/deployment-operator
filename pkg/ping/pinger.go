package ping

import (
	"context"
	"sort"

	ctrclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver/v3"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/polly/containers"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/util"
)

type Pinger struct {
	consoleClient   client.Client
	discoveryClient *discovery.DiscoveryClient
	factory         util.Factory
	k8sClient       ctrclient.Client
}

func New(console client.Client, discovery *discovery.DiscoveryClient, factory util.Factory, k8sClient ctrclient.Client) *Pinger {
	return &Pinger{
		consoleClient:   console,
		discoveryClient: discovery,
		factory:         factory,
		k8sClient:       k8sClient,
	}
}

func (p *Pinger) Ping() error {
	info, err := p.discoveryClient.ServerVersion()
	if err != nil {
		klog.ErrorS(err, "failed to get server version")
		return err
	}

	cs, err := p.factory.KubernetesClientSet()
	if err != nil {
		klog.ErrorS(err, "failed to create kubernetes clientset")
		return nil
	}

	var podNames []string
	var podCount *int64
	// can find some distro information by checking what's running in kube-system
	if pods, err := cs.CoreV1().Pods("kube-system").List(context.Background(), metav1.ListOptions{}); err == nil {
		podNames = lo.Map(pods.Items, func(pod corev1.Pod, ind int) string {
			return pod.Name
		})
	}
	if pods, err := cs.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{}); err == nil {
		podCount = lo.ToPtr(int64(len(pods.Items)))
	}

	minKubeletVersion, azs := p.kubeNodeData(cs)

	var openShiftVersion *string
	apiGroups, err := p.discoveryClient.ServerGroups()
	if err == nil {
		if common.IsRunningOnOpenShift(apiGroups) {
			version, err := common.GetOpenShiftVersion(p.k8sClient)
			if err == nil {
				openShiftVersion = lo.ToPtr(version)
			}
		}
	}

	attrs := pingAttributes(info, podNames, minKubeletVersion, openShiftVersion, podCount)
	attrs.AvailabilityZones = stabilize(azs)
	if err := p.consoleClient.PingCluster(attrs); err != nil {
		attrs.Distro = nil
		return p.consoleClient.PingCluster(attrs) // fallback to no distro to support old console servers
	}

	return nil
}

// kubeNodeData tries to scrape a minimum kubelet version across all nodes in the cluster.
// It is expected that the kubelet will report to the API a valid SemVer-ish version.
// If no parsable version is found across all nodes, nil will be returned.
func (p *Pinger) kubeNodeData(client *kubernetes.Clientset) (*string, containers.Set[string]) {
	azs := containers.NewSet[string]()
	nodes, err := client.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		klog.ErrorS(err, "failed to list nodes")
		return nil, azs
	}

	minKubeletVersion := new(semver.Version)
	for _, node := range nodes.Items {
		if zone, ok := node.GetAnnotations()["topology.kubernetes.io/zone"]; ok {
			azs.Add(zone)
		}

		kubeletVersion, _ := semver.NewVersion(node.Status.NodeInfo.KubeletVersion)
		if kubeletVersion == nil {
			continue
		}

		// Initialize with first correctly parsed version
		if len(minKubeletVersion.Original()) == 0 {
			minKubeletVersion = kubeletVersion
			continue
		}

		if kubeletVersion.LessThan(minKubeletVersion) {
			minKubeletVersion = kubeletVersion
		}
	}

	if len(minKubeletVersion.Original()) == 0 {
		return nil, azs
	}

	return lo.ToPtr(minKubeletVersion.Original()), azs
}

func stabilize(azs containers.Set[string]) []*string {
	azsList := azs.List()
	sort.Strings(azsList)
	return lo.ToSlicePtr(azsList)
}
