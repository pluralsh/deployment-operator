package stacks

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

type socketPublisher struct {
	stackRunQueue workqueue.RateLimitingInterface
	stackRunCache *client.Cache[console.StackRunFragment]
}

func (sp *socketPublisher) Publish(id string) {
	sp.stackRunCache.Expire(id)
	sp.stackRunQueue.Add(id)
}
