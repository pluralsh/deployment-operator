package pipelinegates

import (
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/cache"
)

type socketPublisher struct {
	gateQueue workqueue.TypedRateLimitingInterface[string]
}

func (sp *socketPublisher) Publish(id string) {
	cache.GateCache().Expire(id)
	sp.gateQueue.Add(id)
}
