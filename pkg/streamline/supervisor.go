package streamline

import (
	"context"
	"fmt"
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"

	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/internal/metrics"
	discoverycache "github.com/pluralsh/deployment-operator/pkg/cache/discovery"
	"github.com/pluralsh/deployment-operator/pkg/common"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

var (
	GroupBlacklist = containers.ToSet([]string{
		"aquasecurity.github.io", // Related to compliance/vulnerability reports. Can cause performance issues.
	})

	ResourceVersionBlacklist = containers.ToSet([]string{
		"componentstatuses/v1", // throwing warnings about deprecation since 1.19
		"events/v1",            // no need to watch for resource that are not created by the user
	})

	OptionalResourceVersionList = containers.ToSet([]string{
		"leases/v1", // will be watched dynamically if applier tries to create it
	})
)

type Option func(*Supervisor)

func WithRestartDelay(d time.Duration) Option {
	return func(s *Supervisor) {
		s.restartDelay = d
	}
}

func WithCacheSyncTimeout(d time.Duration) Option {
	return func(s *Supervisor) {
		s.cacheSyncTimeout = d
	}
}

func WithMaxNotFoundRetries(n int) Option {
	return func(s *Supervisor) {
		s.maxNotFoundRetries = n
	}
}

func WithSynchronizerResyncInterval(d time.Duration) Option {
	return func(s *Supervisor) {
		s.synchronizerResyncInterval = d
	}
}

type Supervisor struct {
	mu             sync.RWMutex
	started        bool
	client         dynamic.Interface
	discoveryCache discoverycache.Cache
	store          store.Store

	register chan schema.GroupVersionResource
	done     chan struct{}

	synchronizers              cmap.ConcurrentMap[schema.GroupVersionResource, Synchronizer]
	restartAttemptsLeftMap     cmap.ConcurrentMap[schema.GroupVersionResource, int]
	restartDelay               time.Duration
	cacheSyncTimeout           time.Duration
	synchronizerResyncInterval time.Duration
	maxNotFoundRetries         int
}

func NewSupervisor(client dynamic.Interface, store store.Store, discoveryCache discoverycache.Cache, options ...Option) *Supervisor {
	s := &Supervisor{
		client:                     client,
		discoveryCache:             discoveryCache,
		store:                      store,
		register:                   make(chan schema.GroupVersionResource, 256),
		done:                       make(chan struct{}),
		synchronizers:              cmap.NewStringer[schema.GroupVersionResource, Synchronizer](),
		restartAttemptsLeftMap:     cmap.NewStringer[schema.GroupVersionResource, int](),
		maxNotFoundRetries:         3,
		restartDelay:               1 * time.Second,
		cacheSyncTimeout:           10 * time.Second,
		synchronizerResyncInterval: 30 * time.Minute,
	}

	for _, option := range options {
		option(s)
	}

	return s
}

func (in *Supervisor) WaitForCacheSync(ctx context.Context) error {
	timeoutChan := time.After(in.cacheSyncTimeout)
	syncedCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeoutChan:
			return fmt.Errorf("timed out waiting for cache sync, synced %d out of %d synchronizers", syncedCount, len(in.synchronizers.Items()))
		case <-time.After(time.Second):
			synced := lo.EveryBy(lo.Values(in.synchronizers.Items()), func(s Synchronizer) bool {
				if s.Started() {
					syncedCount++
				}

				return s.Started()
			})

			if synced {
				return nil
			}
		}
	}
}

func (in *Supervisor) MaybeRegister(gvr schema.GroupVersionResource) {
	if OptionalResourceVersionList.Has(fmt.Sprintf("%s/%s", gvr.Resource, gvr.Version)) {
		klog.V(log.LogLevelVerbose).InfoS("skipping resource to watch as it is optional", "gvr", gvr.String())
		return
	}

	in.Register(gvr)
}

func (in *Supervisor) Register(gvr schema.GroupVersionResource) {
	if GroupBlacklist.Has(gvr.Group) || ResourceVersionBlacklist.Has(fmt.Sprintf("%s/%s", gvr.Resource, gvr.Version)) {
		klog.V(log.LogLevelExtended).InfoS("skipping resource to watch as it is blacklisted", "gvr", gvr.String())
		return
	}

	if in.synchronizers.Has(gvr) {
		klog.V(log.LogLevelExtended).InfoS("skipping resource to watch as it is already being watched", "gvr", gvr.String())
		return
	}

	in.mu.RLock()
	if !in.started {
		in.mu.RUnlock()
		return
	}
	in.mu.RUnlock()

	in.resetAttempts(gvr)

	klog.V(log.LogLevelVerbose).InfoS("registering resource to watch", "gvr", gvr.String())
	in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store, in.synchronizerResyncInterval))
	in.enqueue(gvr)
}

func (in *Supervisor) Unregister(gvr schema.GroupVersionResource) {
	if s, ok := in.synchronizers.Get(gvr); ok {
		klog.V(log.LogLevelVerbose).InfoS("unregistering resource from watch", "gvr", gvr.String())
		s.Stop()
		in.synchronizers.Remove(gvr)
	}

	in.clearAttempts(gvr)
}

func (in *Supervisor) Run(ctx context.Context) {
	in.mu.Lock()

	if in.started {
		in.mu.Unlock()
		return
	}

	in.started = true
	in.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				in.stop()
				return
			case gvr, ok := <-in.register:
				if !ok {
					return
				}

				go in.startSynchronizer(ctx, gvr)
			}
		}
	}()

	in.registerInitialResources()
	in.watchDiscoveryCacheChanges()
}

func (in *Supervisor) stop() {
	in.mu.RLock()
	if !in.started {
		in.mu.RUnlock()
		return
	}
	in.mu.RUnlock()

	in.mu.Lock()
	in.started = false
	in.synchronizers.Clear()
	in.restartAttemptsLeftMap.Clear()
	in.mu.Unlock()

	for _, s := range in.synchronizers.Items() {
		s.Stop()
	}

	close(in.done)
	close(in.register)
}

func (in *Supervisor) enqueue(gvr schema.GroupVersionResource) {
	in.mu.RLock()
	if !in.started {
		in.mu.RUnlock()
		return
	}

	ch := in.register
	done := in.done
	in.mu.RUnlock()

	select {
	case ch <- gvr: // enqueued
	case <-done: // shutting down, drop
	default: // buffer full, drop to avoid blocking
		klog.V(log.LogLevelDefault).InfoS("dropping register enqueue due to full buffer", "gvr", gvr.String())
	}
}

func (in *Supervisor) startSynchronizer(ctx context.Context, gvr schema.GroupVersionResource) {
	syn, ok := in.synchronizers.Get(gvr)
	if !ok {
		return
	}

	metrics.Record().ResourceCacheWatchStart(gvr.String())
	err := syn.Start(ctx) // start is a blocking operation
	metrics.Record().ResourceCacheWatchEnd(gvr.String())

	in.synchronizers.Remove(gvr)

	if err == nil {
		klog.V(log.LogLevelVerbose).InfoS("synchronizer stopped", "gvr", gvr.String())
		return
	}

	if apierrors.IsMethodNotSupported(err) {
		klog.V(log.LogLevelVerbose).ErrorS(err, "skipping resource as it is not supported", "gvr", gvr.String())
		return
	}

	if apierrors.IsNotFound(err) {
		left, used := in.decreaseAttempts(gvr)
		if left == 0 {
			klog.V(log.LogLevelVerbose).ErrorS(err, "resource not found after retries, skipping", "gvr", gvr.String(), "attempts", used)
			in.clearAttempts(gvr)
			return
		}

		delay := common.WithJitter(in.restartDelay)
		klog.V(log.LogLevelVerbose).ErrorS(err, "resource not found, retrying", "gvr", gvr.String(), "attemptsLeft", left, "restartDelay", delay)
		in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store, in.synchronizerResyncInterval))
		time.AfterFunc(delay, func() {
			in.enqueue(gvr)
		})

		return
	}

	klog.V(log.LogLevelDefault).ErrorS(err, "unknown synchronizer error, restarting", "gvr", gvr.String())
	in.MaybeRegister(gvr)
}

func (in *Supervisor) registerInitialResources() {
	for _, gvr := range in.discoveryCache.GroupVersionResource().List() {
		in.MaybeRegister(gvr)
	}

	klog.V(log.LogLevelDefault).InfoS("initial resources registered", "count", in.synchronizers.Count())
}

func (in *Supervisor) watchDiscoveryCacheChanges() {
	in.discoveryCache.OnGroupVersionResourceAdded(func(gvr schema.GroupVersionResource) {
		in.MaybeRegister(gvr)
	})

	in.discoveryCache.OnGroupVersionResourceDeleted(func(gvr schema.GroupVersionResource) {
		in.Unregister(gvr)
	})
}

func (in *Supervisor) resetAttempts(gvr schema.GroupVersionResource) {
	in.restartAttemptsLeftMap.Set(gvr, in.maxNotFoundRetries)
}

func (in *Supervisor) decreaseAttempts(gvr schema.GroupVersionResource) (left int, used int) {
	v, ok := in.restartAttemptsLeftMap.Get(gvr)
	if !ok {
		v = in.maxNotFoundRetries
	}

	v--
	if v < 0 {
		v = 0
	}

	in.restartAttemptsLeftMap.Set(gvr, v)
	return v, in.maxNotFoundRetries - v
}

func (in *Supervisor) clearAttempts(gvr schema.GroupVersionResource) {
	in.restartAttemptsLeftMap.Remove(gvr)
}
