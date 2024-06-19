package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

type Controller struct {
	ctx           context.Context
	DynamicClient dynamic.Interface
}

func (c *Controller) Run() error {
	return helpers.BackgroundPollUntilContextCancel(context.TODO(), time.Second*120, true, false, func(ctx context.Context) (done bool, err error) {
		if err != nil {
			// TODO: Log error.
			return false, nil
		}
		for _, gvr := range APIVersions.Items() {
			go func() {
				w, err := c.DynamicClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{})

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

		return false, nil
	})
}
