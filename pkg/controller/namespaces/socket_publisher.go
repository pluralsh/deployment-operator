package namespaces

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/client-go/util/workqueue"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

type socketPublisher struct {
	restoreQueue workqueue.TypedRateLimitingInterface[string]
	restoreCache *client.Cache[console.ManagedNamespaceFragment]
}

func (sp *socketPublisher) Publish(id string) {
	sp.restoreCache.Expire(id)
	sp.restoreQueue.Add(id)
}
