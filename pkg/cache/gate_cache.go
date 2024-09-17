package cache

import (
	"time"

	console "github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/pkg/client"
)

var (
	gateCache *client.Cache[console.PipelineGateFragment]
)

func InitGateCache(expireAfter time.Duration, consoleClient client.Client) {
	gateCache = client.NewCache[console.PipelineGateFragment](expireAfter, func(id string) (*console.PipelineGateFragment, error) {
		return consoleClient.GetClusterGate(id)
	})
}

func GateCache() *client.Cache[console.PipelineGateFragment] {
	if gateCache == nil {
		panic("gate cache is not initialized")
	}

	return gateCache
}
