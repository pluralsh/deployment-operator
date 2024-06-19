package cache

import (
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
)

type cacheLine struct {
	resource string
	created  time.Time
}

func (l *cacheLine) live(dur time.Duration) bool {
	return l.created.After(time.Now().Add(-dur))
}

type Cache struct {
	cache  cmap.ConcurrentMap[string, cacheLine]
	expiry time.Duration
}

func NewCache(expiry time.Duration) *Cache {
	return &Cache{
		cache:  cmap.New[cacheLine](),
		expiry: expiry,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	// Check the cache
	if data, ok := c.cache.Get(key); ok {
		if data.live(c.expiry) {
			return data.resource, true
		}
	}
	return "", false
}

func (c *Cache) Set(key string, value string) {
	c.cache.Set(key, cacheLine{resource: value, created: time.Now()})
}

func (c *Cache) Wipe() {
	c.cache.Clear()
}

func (c *Cache) Expire(key string) {
	c.cache.Remove(key)
}
