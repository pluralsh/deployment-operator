package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var resources = sync.Map{}

type Controller struct {
	k8sClient.Client
	DynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
}

func (c *Controller) Run() error {
	return helpers.BackgroundPollUntilContextCancel(context.TODO(), time.Second*120, true, false, func(ctx context.Context) (done bool, err error) {
		apiGroupsList, err := c.discoveryClient.ServerGroups()
		if err != nil {
			// TODO: Log error.
			return false, nil
		}

		for _, group := range apiGroupsList.Groups {
			for _, version := range group.Versions {
				serverResources, err := c.discoveryClient.ServerResourcesForGroupVersion(version.GroupVersion)
				if err != nil {
					// TODO: Log error.
					return false, nil
				}

				for _, resource := range serverResources.APIResources {
					if !strings.Contains(resource.Name, "/") { // Filter out subresources
						go func() {
							w, err := c.DynamicClient.Resource(schema.GroupVersionResource{
								Group:    resource.Group,
								Version:  resource.Version,
								Resource: "",
							}).Watch(context.TODO(), metav1.ListOptions{})

							if err != nil {
								fmt.Printf("unexpected error establishing watch: %v\n", err)

							}

							for event := range w.ResultChan() {
								switch event.Type {
								case watch.Added, watch.Modified, watch.Deleted:
								default:
									fmt.Printf("unexpected watch event: %#v", event)
								}
							}
						}()
					}
				}
			}
		}
		return false, nil
	})
}
