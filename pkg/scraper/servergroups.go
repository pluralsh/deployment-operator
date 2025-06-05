package scraper

import (
	"context"
	"os"
	"time"

	"github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	"github.com/pluralsh/polly/containers"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/internal/utils"
	"github.com/pluralsh/deployment-operator/pkg/cache"
)

// extraResourcesWatchSet is a set of additional resources that should be watched
// in addition to the core resources. It is used to ensure that we are watching
// resources that are not part of the core Kubernetes API but are still important
var extraResourcesWatchSet = containers.ToSet([]cache.ResourceKey{
	// Cert Manager group
	{GroupKind: runtimeschema.GroupKind{Group: certmanager.GroupName, Kind: ""}, Name: "*", Namespace: "*"},
})

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
		apiGroups, err := discoveryClient.ServerGroups()
		if err != nil {
			klog.ErrorS(err, "failed to get server groups")
			return false, nil
		}

		supportedResources := containers.NewSet[cache.ResourceKey]()
		for _, group := range apiGroups.Groups {
			rk := cache.ResourceKey{
				GroupKind: runtimeschema.GroupKind{
					Group: group.Name,
					Kind:  "", // We are interested in the group only, kind is not specified
				},
				Name:      "*",
				Namespace: "*",
			}

			if extraResourcesWatchSet.Has(rk) {
				// If the resource is in the extra resources watch set, add it to the supported resources
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
