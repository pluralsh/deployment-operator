package restore

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	restoreQueue workqueue.RateLimitingInterface
	restoreCache *client.Cache[console.ClusterRestoreFragment]
}

func (sp *socketPublisher) Publish(id string) {
	sp.restoreCache.Expire(id)
	sp.restoreQueue.Add(id)
}
