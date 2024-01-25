package client

import (
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	console "github.com/pluralsh/console-client-go"
)

type cacheLine[T any] struct {
	svc     *T
	created time.Time
}

type ServiceCache struct {
	client *Client
	cache  cmap.ConcurrentMap[string, *cacheLine[console.ServiceDeploymentExtended]]
	expiry time.Duration
}

func NewCache(client *Client, expiry time.Duration) *ServiceCache {
	return &ServiceCache{
		client: client,
		cache:  cmap.New[*cacheLine[console.ServiceDeploymentExtended]](),
		expiry: expiry,
	}
}

func (c *ServiceCache) Get(id string) (*console.ServiceDeploymentExtended, error) {
	if line, ok := c.cache.Get(id); ok {
		if line.live(c.expiry) {
			return line.svc, nil
		}
	}

	return c.Set(id)
}

func (c *ServiceCache) Set(id string) (*console.ServiceDeploymentExtended, error) {
	svc, err := c.client.GetService(id)
	if err != nil {
		return nil, err
	}

	c.cache.Set(id, &cacheLine[console.ServiceDeploymentExtended]{svc: svc, created: time.Now()})
	return svc, nil
}

func (c *ServiceCache) Wipe() {
	c.cache.Clear()
}

func (c *ServiceCache) Expire(id string) {
	c.cache.Remove(id)
}

func (l *cacheLine[T]) live(dur time.Duration) bool {
	return l.created.After(time.Now().Add(-dur))
}
