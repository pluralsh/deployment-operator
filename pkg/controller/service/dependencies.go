package service

import (
	"sync"

	console "github.com/pluralsh/console/go/client"
)

var (
	servicePresent = make(map[string][]*console.ServiceDependencyFragment)
	cacheMu        sync.RWMutex
)

func (s *ServiceReconciler) registerDependencies(svc *console.ServiceDeploymentForAgent) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Update or add the service with its latest dependencies
	servicePresent[svc.Name] = svc.Dependencies

	// Sideload dependencies: ensure they exist in the map
	for _, dep := range svc.Dependencies {
		if _, exists := servicePresent[dep.Name]; !exists {
			depSvc, err := s.svcCache.Get(dep.ID)
			if err != nil && depSvc != nil && depSvc.DeletedAt == nil {
				servicePresent[dep.Name] = depSvc.Dependencies
			}
		}
	}
}

// getActiveDependents returns a list of service names that depend on the given service
func getActiveDependents(svcName string) []string {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	var dependents []string
	for name, deps := range servicePresent {
		if name == svcName {
			continue
		}
		for _, d := range deps {
			if d.Name == svcName {
				dependents = append(dependents, name)
				break
			}
		}
	}
	return dependents
}

// removeService removes a service from the map
func removeService(svcName string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	delete(servicePresent, svcName)
}
