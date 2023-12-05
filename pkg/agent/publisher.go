package agent

import (
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"

	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	svcQueue workqueue.RateLimitingInterface
	svcCache *client.ServiceCache
	manCache *manifests.ManifestCache
}

func (pub *socketPublisher) PublishService(id string) {
	pub.svcCache.Expire(id)
	pub.manCache.Expire(id)
	pub.svcQueue.Add(id)
}
