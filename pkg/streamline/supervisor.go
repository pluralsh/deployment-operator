package streamline

import (
	"context"
	"fmt"
	"math/rand"
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
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/streamline/store"
)

const (
	defaultStartJitter        = 2 * time.Second
	defaultMinStartDelay      = 500 * time.Millisecond
	defaultMaxNotFoundRetries = 3
)

var (
	supervisorMu sync.Mutex
	supervisor   *Supervisor

	GroupBlacklist = containers.ToSet([]string{
		"aquasecurity.github.io", // Related to compliance/vulnerability reports. Can cause performance issues.
	})

	ResourceVersionBlacklist = containers.ToSet([]string{
		"componentstatuses/v1", // throwing warnings about deprecation since 1.19
		"events/v1",            // throwing unique constraint violation when trying to store in cache
	})
)

type Supervisor struct {
	mu                     sync.RWMutex
	client                 dynamic.Interface
	discoveryCache         discoverycache.Cache
	store                  store.Store
	register               chan schema.GroupVersionResource
	synchronizers          cmap.ConcurrentMap[schema.GroupVersionResource, Synchronizer]
	restartAttemptsLeftMap cmap.ConcurrentMap[schema.GroupVersionResource, int]

	// TODO: make configurable through args
	startJitter        time.Duration
	maxNotFoundRetries int
}

func Run(ctx context.Context, client dynamic.Interface, store store.Store, discoveryCache discoverycache.Cache) {
	supervisorMu.Lock()

	if supervisor != nil {
		return
	}

	supervisor = &Supervisor{
		client:                 client,
		discoveryCache:         discoveryCache,
		store:                  store,
		register:               make(chan schema.GroupVersionResource),
		synchronizers:          cmap.NewStringer[schema.GroupVersionResource, Synchronizer](),
		restartAttemptsLeftMap: cmap.NewStringer[schema.GroupVersionResource, int](),
		startJitter:            defaultStartJitter,
		maxNotFoundRetries:     defaultMaxNotFoundRetries,
	}

	supervisorMu.Unlock()

	supervisor.run(ctx)
}

func WaitForCacheSync(ctx context.Context) error {
	// TODO: make this configurable through args
	const timeout = 10 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("timed out waiting for cache sync")
		case <-time.After(time.Second):
			synced := lo.EveryBy(lo.Values(supervisor.synchronizers.Items()), func(s Synchronizer) bool {
				return s.Started()
			})

			if synced {
				return nil
			}
		}
	}
}

func (in *Supervisor) Register(gvr schema.GroupVersionResource) {
	if GroupBlacklist.Has(gvr.Group) || ResourceVersionBlacklist.Has(fmt.Sprintf("%s/%s", gvr.Resource, gvr.Version)) {
		klog.V(log.LogLevelVerbose).InfoS("skipping resource to watch as it is blacklisted", "gvr", gvr.String())
		return
	}

	in.mu.Lock()
	defer in.mu.Unlock()

	if in.synchronizers.Has(gvr) {
		klog.V(log.LogLevelVerbose).InfoS("skipping resource to watch as it is already being watched", "gvr", gvr.String())
		return
	}

	in.resetAttempts(gvr)

	startDelay := in.startDelay()
	klog.V(log.LogLevelVerbose).InfoS("registering resource to watch", "gvr", gvr.String(), "startDelay", startDelay)
	in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store))
	time.AfterFunc(startDelay, func() {
		in.register <- gvr
	})
}

func (in *Supervisor) Unregister(gvr schema.GroupVersionResource) {
	in.mu.Lock()
	defer in.mu.Unlock()

	if s, ok := in.synchronizers.Get(gvr); ok {
		klog.V(log.LogLevelVerbose).InfoS("unregistering resource from watch", "gvr", gvr.String())
		s.Stop()
		in.synchronizers.Remove(gvr)
	}

	in.clearAttempts(gvr)
}

func (in *Supervisor) run(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				in.stop()
				return
			case gvr := <-in.register:
				go in.startSynchronizer(ctx, gvr)
			}
		}
	}()

	in.registerInitialResources()
	in.watchDiscoveryCacheChanges()
}

func (in *Supervisor) stop() {
	in.mu.Lock()
	defer in.mu.Unlock()

	for _, s := range in.synchronizers.Items() {
		s.Stop()
	}

	in.synchronizers.Clear()
	in.restartAttemptsLeftMap.Clear()
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

		delay := in.startDelay()
		klog.V(log.LogLevelVerbose).ErrorS(err, "resource not found, retrying", "gvr", gvr.String(), "attemptsLeft", left, "delay", delay)
		in.synchronizers.Set(gvr, NewSynchronizer(in.client, gvr, in.store))
		time.AfterFunc(delay, func() {
			in.register <- gvr
		})

		return
	}

	klog.V(log.LogLevelDefault).ErrorS(err, "unknown synchronizer error, restarting", "gvr", gvr.String())
	in.Register(gvr)
}

func (in *Supervisor) startDelay() time.Duration {
	jitter := time.Duration(rand.Int63n(int64(in.startJitter)) - int64(in.startJitter/2))
	if jitter < 0 {
		jitter = 0
	}

	return defaultMinStartDelay + jitter
}

func (in *Supervisor) registerInitialResources() {
	for _, gvr := range in.discoveryCache.GroupVersionResource().List() {
		in.Register(gvr)
	}
}

func (in *Supervisor) watchDiscoveryCacheChanges() {
	in.discoveryCache.OnGroupVersionResourceAdded(func(gvr schema.GroupVersionResource) {
		in.Register(gvr)
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
