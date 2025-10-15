package sentinel

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

type socketPublisher struct {
	sentinelRunQueue workqueue.TypedRateLimitingInterface[string]
	sentinelRunCache *client.Cache[console.SentinelRunJobFragment]
}

func (sp *socketPublisher) Publish(id string, _ bool) {
	sp.sentinelRunCache.Expire(id)
	sp.sentinelRunQueue.Add(id)
}
