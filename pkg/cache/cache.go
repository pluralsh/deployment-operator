package cache

import (
	"context"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
)

type Expirable interface {
	Expire()
}

type cacheLine[T Expirable] struct {
	resource T
	created  time.Time
}

func (l *cacheLine[_]) alive(ttl time.Duration) bool {
	return l.created.After(time.Now().Add(-ttl))
}

type Cache[T Expirable] struct {
	cache                  cmap.ConcurrentMap[string, cacheLine[T]]
	ttl                    time.Duration
	expirationPollInterval time.Duration
	ctx                    context.Context
}

func NewCache[T Expirable](ctx context.Context, ttl time.Duration) *Cache[T] {
	return &Cache[T]{
			cache: cmap.New[cacheLine[T]](),
			ttl:   ttl,
			ctx:   ctx,
		}
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
	expirable, exists := c.cache.Get(key)
	if !exists {
		return
	}

	expirable.resource.Expire()
	expirable.created = time.Now()
	c.cache.Set(key, expirable)
}

func (c *Cache[T]) Clean() {}
