package service

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/manifests"

	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	svcQueue workqueue.RateLimitingInterface
	svcCache *client.Cache[console.ServiceDeploymentExtended]
	manCache *manifests.ManifestCache
}

func (pub *socketPublisher) PublishService(id string) {
	pub.svcCache.Expire(id)
	pub.manCache.Expire(id)
	pub.svcQueue.Add(id)
}
