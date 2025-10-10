package stacks

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

type socketPublisher struct {
	stackRunQueue workqueue.TypedRateLimitingInterface[string]
	stackRunCache *client.Cache[console.SentinelRunJobFragment]
}

func (sp *socketPublisher) Publish(id string, _ bool) {
	sp.stackRunCache.Expire(id)
	sp.stackRunQueue.Add(id)
}
