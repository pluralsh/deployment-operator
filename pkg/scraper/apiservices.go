package scraper

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/polly/containers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	apiclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

var apiServices *ApiServices

func init() {
	apiServices = &ApiServices{
		apiServices: containers.NewSet[string](),
	}
}

type ApiServices struct {
	mu          sync.RWMutex
	apiServices containers.Set[string]
}

func GetApiServices() *ApiServices {
	return apiServices
}

func (in *ApiServices) Add(name string) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.apiServices.Add(name)
}
func (in *ApiServices) List() []string {
	in.mu.RLock()
	defer in.mu.RUnlock()
	return in.apiServices.List()
}

func RunAPIServicesScraperInBackgroundOrDie(ctx context.Context, config *rest.Config) {
	apiClient, err := apiclient.NewForConfig(config)
	if err != nil {
		panic(fmt.Errorf("failed to create API client: %w", err))
	}

	interval := func() time.Duration { return time.Minute }

	_ = helpers.DynamicBackgroundPollUntilContextCancel(ctx, interval, false, func(_ context.Context) (done bool, err error) {
		apiservices, err := apiClient.ApiregistrationV1().APIServices().List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, apiservice := range apiservices.Items {
				if apiservice.Spec.Group != "" {
					GetApiServices().Add(fmt.Sprintf("%s/%s", apiservice.Spec.Group, apiservice.Spec.Version))
				}
			}
		}

		return false, nil
	})

}
