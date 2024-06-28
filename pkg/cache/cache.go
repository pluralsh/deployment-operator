package cache

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
)

type cacheLine[T any] struct {
	resource T
	created  time.Time
}

func (l *cacheLine[_]) alive(ttl time.Duration) bool {
	return l.created.After(time.Now().Add(-ttl))
}

type Cache[T any] struct {
	cache                  cmap.ConcurrentMap[string, cacheLine[T]]
	ttl                    time.Duration
	expirationPollInterval time.Duration
	ctx context.Context
}

func (c *Cache[T]) init() *Cache[T] {
	go wait.PollUntilContextCancel(
		c.ctx,
		c.expirationPollInterval,
		false,
		func(_ context.Context) (done bool, err error) {
			for k, v := range c.cache.Items() {
				if !v.alive(c.ttl) {
					c.Expire(k)
				}
			}
			return false, nil
		})

	return c
}

func NewCache[T any](ctx context.Context, ttl, expirationPollInterval time.Duration) *Cache[T] {
	return (&Cache[T]{
		cache:                  cmap.New[cacheLine[T]](),
		ttl:                    ttl,
		expirationPollInterval: expirationPollInterval,
		ctx: ctx,
	}).init()
}

func (c *Cache[T]) Get(key string) (T, bool) {
	data, ok := c.cache.Get(key)
	if ok && data.alive(c.ttl) {
		return data.resource, true
	}

	c.Expire(key)
	return lo.Empty[T](), false
}

func (c *Cache[T]) Set(key string, value T) {
	c.cache.Set(key, cacheLine[T]{resource: value, created: time.Now()})
}

func (c *Cache[T]) Wipe() {
	c.cache.Clear()
}

func (c *Cache[T]) Expire(key string) {
	c.cache.Remove(key)
}

func (c *Cache[T]) Clean() {}
