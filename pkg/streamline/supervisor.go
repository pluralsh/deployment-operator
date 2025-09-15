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
	supervisorMu sync.Mutex
	supervisor   *Supervisor

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
	mu                         sync.RWMutex
	started                    bool
	client                     dynamic.Interface
	discoveryCache             discoverycache.Cache
	store                      store.Store
	register                   chan schema.GroupVersionResource
	synchronizers              cmap.ConcurrentMap[schema.GroupVersionResource, Synchronizer]
	restartAttemptsLeftMap     cmap.ConcurrentMap[schema.GroupVersionResource, int]
	restartDelay               time.Duration
	cacheSyncTimeout           time.Duration
	synchronizerResyncInterval time.Duration
	maxNotFoundRetries         int
}

func Run(ctx context.Context, client dynamic.Interface, store store.Store, discoveryCache discoverycache.Cache, options ...Option) {
	if supervisor != nil {
		return
	}

	supervisorMu.Lock()

	supervisor = &Supervisor{
		client:                     client,
		discoveryCache:             discoveryCache,
		store:                      store,
		register:                   make(chan schema.GroupVersionResource),
		synchronizers:              cmap.NewStringer[schema.GroupVersionResource, Synchronizer](),
		restartAttemptsLeftMap:     cmap.NewStringer[schema.GroupVersionResource, int](),
		maxNotFoundRetries:         3,
		restartDelay:               1 * time.Second,
		cacheSyncTimeout:           10 * time.Second,
		synchronizerResyncInterval: 30 * time.Minute,
	}

	for _, option := range options {
		option(supervisor)
	}

	supervisorMu.Unlock()

	supervisor.run(ctx)
}

func WaitForCacheSync(ctx context.Context) error {
	if supervisor == nil {
		return fmt.Errorf("supervisor is not running")
	}

	timeoutChan := time.After(supervisor.cacheSyncTimeout)
	syncedCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timeoutChan:
			return fmt.Errorf("timed out waiting for cache sync, synced %d out of %d synchronizers", syncedCount, len(supervisor.synchronizers.Items()))
		case <-time.After(time.Second):
			synced := lo.EveryBy(lo.Values(supervisor.synchronizers.Items()), func(s Synchronizer) bool {
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

func GetSupervisor() *Supervisor {
	supervisorMu.Lock()
	defer supervisorMu.Unlock()

	return supervisor
}

func (in *Supervisor) MaybeRegister(gvr schema.GroupVersionResource) {
	if OptionalResourceVersionList.Has(fmt.Sprintf("%s/%s", gvr.Resource, gvr.Version)) {
		klog.V(log.LogLevelVerbose).InfoS("skipping resource to watch as it is optional", "gvr", gvr.String())
		return
	}

	in.Register(gvr)
}

func (in *Supervisor) Register(gvr schema.GroupVersionResource) {
	if !in.started {
		return
	}

	if GroupBlacklist.Has(gvr.Group) || ResourceVersionBlacklist.Has(fmt.Sprintf("%s/%s", gvr.Resource, gvr.Version)) {
		klog.V(log.LogLevelExtended).InfoS("skipping resource to watch as it is blacklisted", "gvr", gvr.String())
		return
	}

	if in.synchronizers.Has(gvr) {
		klog.V(log.LogLevelExtended).InfoS("skipping resource to watch as it is already being watched", "gvr", gvr.String())
		return
	}

	in.resetAttempts(gvr)

	klog.V(log.LogLevelVerbose).InfoS("registering resource to watch", "gvr", gvr.String())
	in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store, in.synchronizerResyncInterval))
	in.register <- gvr
}

func (in *Supervisor) Unregister(gvr schema.GroupVersionResource) {
	if s, ok := in.synchronizers.Get(gvr); ok {
		klog.V(log.LogLevelVerbose).InfoS("unregistering resource from watch", "gvr", gvr.String())
		s.Stop()
		in.synchronizers.Remove(gvr)
	}

	in.clearAttempts(gvr)
}

func (in *Supervisor) run(ctx context.Context) {
	in.mu.Lock()

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

	in.started = true
	in.mu.Unlock()

	in.registerInitialResources()
	in.watchDiscoveryCacheChanges()
}

func (in *Supervisor) stop() {
	in.mu.Lock()
	defer in.mu.Unlock()

	if in.started == false {
		return
	}

	for _, s := range in.synchronizers.Items() {
		s.Stop()
	}

	in.synchronizers.Clear()
	in.restartAttemptsLeftMap.Clear()
	in.started = false
	close(in.register)
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
			in.register <- gvr
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
