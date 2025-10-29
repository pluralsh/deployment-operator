package service

import (
	"sync"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/containers"
)

var (
	allServices    = containers.NewSet[string]()
	servicePresent = make(map[console.ServiceDependencyFragment][]*console.ServiceDependencyFragment)
	cacheMu        sync.RWMutex
)

func (s *ServiceReconciler) registerDependencies(svc *console.ServiceDeploymentForAgent) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Update or add the service with its latest dependencies
	servicePresent[console.ServiceDependencyFragment{Name: svc.Name, ID: svc.ID}] = svc.Dependencies

	// Sideload dependencies: ensure they exist in the map
	for _, dep := range svc.Dependencies {
		if _, exists := servicePresent[*dep]; !exists {
			depSvc, err := s.svcCache.Get(dep.ID)
			if err != nil && depSvc != nil && depSvc.DeletedAt == nil {
				servicePresent[*dep] = depSvc.Dependencies
			}
		}
	}
}

// getActiveDependents returns a list of service names that depend on the given service
func (s *ServiceReconciler) getActiveDependents(svcName string) []string {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	var dependents []string
	for service, deps := range servicePresent {
		if service.Name == svcName {
			continue
		}
		for _, d := range deps {
			if d.Name == svcName {
				// check if the service is still in the cache
				// it could have been detached
				if allServices.Has(service.ID) {
					dependents = append(dependents, service.Name)
					break
				}
			}
		}
	}

	return dependents
}

// unregisterDependencies removes a service from the map
func unregisterDependencies(svc *console.ServiceDeploymentForAgent) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	delete(servicePresent, console.ServiceDependencyFragment{Name: svc.Name, ID: svc.ID})
}

func addService(id string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	allServices.Add(id)
}

func cleanupServices() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	allServices = containers.NewSet[string]()
}
