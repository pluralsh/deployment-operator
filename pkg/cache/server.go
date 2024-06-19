package cache

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/pluralsh/deployment-operator/internal/helpers"
)

type ServerCache struct {
	ctx           context.Context
	dynamicClient dynamic.Interface
	Cache         *Cache
}

func NewServerCache(ctx context.Context, dynamicClient dynamic.Interface, expiry time.Duration) *ServerCache {
	return &ServerCache{
		ctx:           ctx,
		dynamicClient: dynamicClient,
		Cache:         NewCache(expiry),
	}
}

func (c *ServerCache) Run() error {
	return helpers.BackgroundPollUntilContextCancel(context.TODO(), time.Second*120, true, false, func(ctx context.Context) (done bool, err error) {
		//if err != nil {
		//	// TODO: Log error.
		//	return false, nil
		//}
		//for _, gvr := range APIVersions.Items() {
		//	w, err := c.dynamicClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{
		//		FieldSelector: fmt.Sprintf("metadata.annotations.\"config.k8s.io/owning-inventory\"!=\"\""),
		//	})
		//	if err != nil {
		//		fmt.Printf("unexpected error establishing watch: %v\n", err)
		//		continue
		//	}
		//	go c.Reconcile(w.ResultChan())
		//}
		return false, nil
	})
}

func (c *ServerCache) Reconcile(echan <-chan watch.Event) {
	for event := range echan {
		switch event.Type {
		case watch.Added, watch.Modified, watch.Deleted:
			fmt.Println("changed")
		default:
			fmt.Printf("unexpected watch event: %#v\n", event)
		}
	}
}
