package scraper

import (
	"context"
	"os"
	"time"

	"github.com/pluralsh/polly/containers"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

const (
	serverGroupsScraperPollingInterval = 15 * time.Minute
)

func RunServerGroupsScraperInBackgroundOrDie(ctx context.Context, config *rest.Config) {
	f := utils.NewFactory(config)
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		klog.Error(err, "unable to create discovery client")
		os.Exit(1)
	}

	// Start background polling to check if CRDs for extra resources exist
	err = helpers.BackgroundPollUntilContextCancel(ctx, serverGroupsScraperPollingInterval, false, true, func(_ context.Context) (done bool, err error) {
		resources, err := discoveryClient.ServerPreferredResources()
		if err != nil {
			klog.ErrorS(err, "failed to get server groups and resources")
			return false, nil
		}

		supportedResources := containers.NewSet[cache.ResourceKey]()
		for _, l := range resources {
			if l == nil {
				continue
			}

			for _, resource := range l.APIResources {
				rk := cache.ResourceKey{
					GroupKind: runtimeschema.GroupKind{
						Group: resource.Group,
						Kind:  resource.Kind,
					},
					Name:      "*",
					Namespace: "*",
				}

				// try watching everything
				supportedResources.Add(rk)
			}
		}

		// Register the supported resources from extra resources
		if supportedResources.Len() > 0 {
			cache.GetResourceCache().Register(supportedResources)
		}

		return false, nil
	})
	if err != nil {
		klog.ErrorS(err, "failed to start extra resources watch in background")
		os.Exit(1)
	}
}
