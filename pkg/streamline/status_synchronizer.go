package streamline

import (
	"context"
	"slices"
	"strings"
	"time"

	"golang.org/x/time/rate"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/cache"
	"github.com/pluralsh/deployment-operator/pkg/common"
)

type StatusSynchronizer struct {
	client      console.Client
	shaCache    *cache.SimpleCache[string]
	rateLimiter *rate.Limiter
}

// NewStatusSynchronizer creates a new StatusSynchronizer with rate limiting set to 10 calls per second.
func NewStatusSynchronizer(client console.Client, cacheTTL time.Duration) *StatusSynchronizer {
	return &StatusSynchronizer{
		client:      client,
		shaCache:    cache.NewSimpleCache[string](cacheTTL),
		rateLimiter: rate.NewLimiter(rate.Limit(10), 1),
	}
}

func (in *StatusSynchronizer) UpdateServiceComponents(ctx context.Context, serviceId, revisionId string, components []*console.ComponentAttributes) error {
	// Ensure consistent ordering for comparison.
	slices.SortFunc(components, func(a, b *console.ComponentAttributes) int {
		return strings.Compare(common.ComponentAttributesKey(*a), common.ComponentAttributesKey(*b))
	})

	// Hash the components to determine if there has been a meaningful change we need to report to the server.
	sha, err := utils.HashObject(struct {
		ServiceId  string                         `json:"serviceId"`
		RevisionId string                         `json:"revisionId"`
		Attributes []*console.ComponentAttributes `json:"attributes"`
	}{
		ServiceId:  serviceId,
		RevisionId: revisionId,
		Attributes: components,
	})
	if err != nil {
		return err
	}

	if old, ok := in.shaCache.Get(serviceId); ok && old == sha {
		return nil
	}

	// Rate limit API calls. If the rate limit is exceeded, skip the update silently.
	if !in.rateLimiter.Allow() {
		return nil
	}

	if _, err = in.client.UpdateServiceComponents(ctx, serviceId, components, revisionId, nil, nil, nil); err != nil {
		return err
	}

	in.shaCache.Add(serviceId, sha)

	return nil
}
