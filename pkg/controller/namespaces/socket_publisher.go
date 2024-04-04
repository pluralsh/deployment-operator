package namespaces

import (
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"k8s.io/client-go/util/workqueue"
)

type socketPublisher struct {
	restoreQueue workqueue.RateLimitingInterface
	restoreCache *client.Cache[console.ManagedNamespaceFragment]
}

func (sp *socketPublisher) Publish(id string) {
	sp.restoreCache.Expire(id)
	sp.restoreQueue.Add(id)
}
