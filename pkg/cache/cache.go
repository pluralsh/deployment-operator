package cache

import (
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
}

func NewCache[T any](ttl, expirationPollInterval time.Duration) *Cache[T] {
	/*	go wait.PollUntilContextCancel(
		context.Background(),
		expirationPollInterval,
		false,
		func(ctx context.Context) (done bool, err error) {

			return false, nil
		})*/

	return &Cache[T]{
		cache:                  cmap.New[cacheLine[T]](),
		ttl:                    ttl,
		expirationPollInterval: expirationPollInterval,
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
	c.cache.Remove(key)
}

func (c *Cache[T]) Clean() {}
