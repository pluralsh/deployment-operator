package cache

import (
	"context"
	"fmt"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/metrics"
)

var (
	// Maps a GroupVersion to resource Kind.
	discoveryCache cmap.ConcurrentMap[string, bool]
)

func init() {
	discoveryCache = cmap.New[bool]()
}

func DiscoveryCache() cmap.ConcurrentMap[string, bool] {
	return discoveryCache
}

func RunDiscoveryCacheInBackgroundOrDie(ctx context.Context, discoveryClient *discovery.DiscoveryClient) {
	klog.Info("starting discovery cache")
	err := helpers.BackgroundPollUntilContextCancel(ctx, 5*time.Minute, true, true, func(_ context.Context) (done bool, err error) {
		if err = updateDiscoveryCache(discoveryClient); err != nil {
			klog.Error(err, "can't fetch API versions")
		}

		metrics.Record().DiscoveryAPICacheRefresh(err)
		return false, nil
	})
	if err != nil {
		panic(fmt.Errorf("failed to start discovery cache in background: %w", err))
	}
}

func updateDiscoveryCache(discoveryClient *discovery.DiscoveryClient) error {
	lists, err := discoveryClient.ServerPreferredResources()

	for _, list := range lists {
		if len(list.APIResources) == 0 {
			continue
		}

		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}

			discoveryCache.Set(fmt.Sprintf("%s/%s", gv.String(), resource.Kind), true)
		}
	}

	return err
}
