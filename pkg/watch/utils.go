package watch

import (
	"strings"

	"github.com/pluralsh/polly/containers"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func getAllResourceTypes(discoveryClient *discovery.DiscoveryClient) (containers.Set[schema.GroupVersionResource], error) {
	result := containers.NewSet[schema.GroupVersionResource]()
	lists, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}
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
			if strings.Contains(resource.Name, "/") {
				continue
			}

			if !contains(resource.Verbs, "watch") {
				continue
			}
			result.Add(schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: resource.Name})
		}
	}
	return result, nil
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
