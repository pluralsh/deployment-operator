package pipelinegates

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/client"

	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	gateQueue workqueue.RateLimitingInterface
	gateCache *client.Cache[console.PipelineGateFragment]
}

func (sp *socketPublisher) Publish(id string) {
	sp.gateCache.Expire(id)
	sp.gateQueue.Add(id)
}
