package cache

import (
	"context"
	"math/rand"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/agent/args"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

type Expirable interface {
	Expire()
}

var defaultJitter = args.RefreshInterval() + time.Minute

type cacheLine[T Expirable] struct {
	resource T
	created  time.Time
}

func (l *cacheLine[_]) alive(ttl time.Duration) bool {
	return l.created.After(time.Now().Add(-ttl))
}

type Cache[T Expirable] struct {
	cache  cmap.ConcurrentMap[string, cacheLine[T]]
	ttl    time.Duration
	jitter time.Duration
	ctx    context.Context
	mux    sync.Mutex
}

func NewCache[T Expirable](ctx context.Context, ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		cache:  cmap.New[cacheLine[T]](),
		ttl:    ttl,
		jitter: defaultJitter,
		ctx:    ctx,
	}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	data, exists := c.cache.Get(key)
	if !exists {
		return lo.Empty[T](), false
	}

	if !data.alive(c.ttl) {
		c.Expire(key)
	}

	return data.resource, true
}

func (c *Cache[T]) Set(key string, value T) {
	c.cache.Set(key, cacheLine[T]{resource: value, created: c.createdAt()})
}

func (c *Cache[T]) SetPreservingAge(key string, value T) {
	c.cache.Upsert(
		key,
		cacheLine[T]{resource: value},
		func(exist bool, valueInMap, newValue cacheLine[T]) cacheLine[T] {
			return lo.Ternary(
				exist,
				cacheLine[T]{newValue.resource, valueInMap.created},
				cacheLine[T]{newValue.resource, c.createdAt()},
			)
		})
}

func (c *Cache[T]) Wipe() {
	c.cache.Clear()
}

func (c *Cache[T]) Expire(key string) {
	c.mux.Lock()
	defer c.mux.Unlock()

	expirable, exists := c.cache.Get(key)
	if !exists {
		return
	}

	klog.V(log.LogLevelDebug).InfoS("expiring resource cache entry", "key", key, "resource", expirable.resource, "created", expirable.created)
	expirable.resource.Expire()
	expirable.created = c.createdAt()
	c.cache.Set(key, expirable)
}

func (c *Cache[T]) createdAt() time.Time {
	return time.Now().Add(-time.Duration(rand.Int63n(int64(c.jitter))))
}
