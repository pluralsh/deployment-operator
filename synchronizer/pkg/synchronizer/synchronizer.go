package synchronizer

import (
	"time"

	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
	"github.com/pluralsh/deployment-operator/common/log"
	"github.com/pluralsh/deployment-operator/synchronizer/pkg/console"
	"github.com/pluralsh/deployment-operator/synchronizer/pkg/kubernetes"
)

type Synchronizer struct {
	consoleClient    *console.Client
	kubernetesClient kubernetes.Client
	interval         time.Duration
}

func New(url, token string, interval time.Duration) Synchronizer {
	return Synchronizer{
		consoleClient:    console.New(url, token),
		kubernetesClient: kubernetes.New(),
		interval:         interval,
	}
}

func (s *Synchronizer) Run() {
	log.Logger.Info("Starting synchronizer...")

	for {
		apiServices, err := s.consoleClient.GetServices()
		if err != nil {
			log.Logger.Error(err, "failed to fetch service list from deployments service")
			time.Sleep(s.interval)
			continue
		}

		services := toKubernetesServices(apiServices)
		existingServices, err := s.kubernetesClient.GetServices()
		if err != nil {
			log.Logger.Error(err, "failed to fetch service list from cluster")
			time.Sleep(s.interval)
			continue
		}

		err = s.sync(services, existingServices.Items)
		if err != nil {
			log.Logger.Error(err, "failed to sync services from deployments service to cluster")
			time.Sleep(s.interval)
			continue
		}

		time.Sleep(s.interval)
	}
}

// TODO: Should we exit on first error?
func (s *Synchronizer) sync(services, existingServices []platform.Deployment) error {
	existingServicesMap := map[string]platform.Deployment{}
	for _, svc := range existingServices {
		existingServicesMap[svc.Name] = svc
	}

	for _, svc := range services {
		if _, exists := existingServicesMap[svc.Name]; exists {
			err := s.kubernetesClient.UpdateService(svc)
			if err != nil {
				return err
			}
		} else {
			err := s.kubernetesClient.CreateService(svc)
			if err != nil {
				return err
			}
		}

		delete(existingServicesMap, svc.Name)
	}

	// TODO: Remove as it should be possible to create CRDs outside API?
	for _, svc := range existingServicesMap {
		err := s.kubernetesClient.DeleteService(svc)
		if err != nil {
			return err
		}
	}

	return nil
}
