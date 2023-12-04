package client

import (
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type genericCacheLine[T any] struct {
	item    *T
	created time.Time
}

type GenericCache[T any] struct {
	client *Client
	// retrieval function that takes an id and returns a T
	retrieve func(*Client, string) (*T, error)
	cache    cmap.ConcurrentMap[string, *genericCacheLine[T]]
	expiry   time.Duration
}

func NewGenericCache[T any](client *Client, retrieve func(*Client, string) (*T, error), expiry time.Duration) *GenericCache[T] {
	return &GenericCache[T]{
		client:   client,
		retrieve: retrieve,
		cache:    cmap.New[*genericCacheLine[T]](),
		expiry:   expiry,
	}
}

func (c *GenericCache[T]) Get(id string) (*T, error) {
	if line, ok := c.cache.Get(id); ok {
		if line.live(c.expiry) {
			return line.item, nil
		}
	}

	return c.Set(id)
}

func (c *GenericCache[T]) Set(id string) (*T, error) {
	//item, err := c.client.GetService(id)
	item, err := c.retrieve(c.client, id)
	if err != nil {
		return nil, err
	}

	//c.cache.Set(id, &genericCacheLine{item: item, created: time.Now()})
	c.cache.Set(id, &genericCacheLine[T]{item: item, created: time.Now()})
	return item, nil
}

func (c *GenericCache[T]) Wipe() {
	c.cache.Clear()
}

func (c *GenericCache[T]) Expire(id string) {
	c.cache.Remove(id)
}

func (l *genericCacheLine[T]) live(dur time.Duration) bool {
	return l.created.After(time.Now().Add(-dur))
}
