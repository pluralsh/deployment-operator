package service

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"
	"github.com/pluralsh/deployment-operator/pkg/streamline"

	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	svcQueue workqueue.TypedRateLimitingInterface[string]
	svcCache *client.Cache[console.ServiceDeploymentForAgent]
	manCache *manifests.ManifestCache
}

func (sp *socketPublisher) Publish(id string, kick bool) {
	sp.svcCache.Expire(id)
	sp.manCache.Expire(id)
	if kick {
		err := streamline.GetGlobalStore().Expire(id)
		if err != nil {
			klog.ErrorS(err, "unable to expire service", "id", id)
		}
	}

	sp.svcQueue.Add(id)
}
