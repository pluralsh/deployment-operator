package common

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pluralsh/deployment-operator/api/v1alpha1"
)

func init() {
	configurationManager = &ConfigurationManager{}
}

var configurationManager *ConfigurationManager

// Configuration is a thread-safe structure for agent configuration
type ConfigurationManager struct {
	mu                          sync.RWMutex
	servicePollInterval         *time.Duration
	clusterPingInterval         *time.Duration
	runtimeServicesPingInterval *time.Duration
	stackPollInterval           *time.Duration
	compatibilityUploadInterval *time.Duration
	pipelineGateInterval        *time.Duration
	maxConcurrentReconciles     *int
	baseRegistryURL             *string
}

func GetConfigurationManager() *ConfigurationManager {
	return configurationManager
}

// SetValue sets the value of the string in a thread-safe manner
func (s *ConfigurationManager) SetValue(config v1alpha1.AgentConfigurationSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	interval, err := setDuration(config.ClusterPingInterval)
	if err != nil {
		return err
	}
	s.clusterPingInterval = interval

	interval, err = setDuration(config.CompatibilityUploadInterval)
	if err != nil {
		return err
	}
	s.runtimeServicesPingInterval = interval

	interval, err = setDuration(config.PipelineGateInterval)
	if err != nil {
		return err
	}
	s.pipelineGateInterval = interval

	interval, err = setDuration(config.StackPollInterval)
	if err != nil {
		return err
	}
	s.stackPollInterval = interval

	interval, err = setDuration(config.VulnerabilityReportUploadInterval)
	if err != nil {
		return err
	}
	s.compatibilityUploadInterval = interval

	interval, err = setDuration(config.ServicePollInterval)
	if err != nil {
		return err
	}
	s.servicePollInterval = interval

	s.maxConcurrentReconciles = config.MaxConcurrentReconciles
	s.baseRegistryURL = config.BaseRegistryURL

	return nil
}

func setDuration(interval *string) (*time.Duration, error) {
	if interval == nil {
		return nil, nil
	}
	duration, err := time.ParseDuration(*interval)
	if err != nil {
		return nil, err
	}
	return &duration, nil
}

func (s *ConfigurationManager) GetClusterPingInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clusterPingInterval
}

func (s *ConfigurationManager) GetRuntimeServicesPingInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeServicesPingInterval
}

func (s *ConfigurationManager) GetCompatibilityUploadInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compatibilityUploadInterval
}

func (s *ConfigurationManager) GetPipelineGateInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pipelineGateInterval
}

func (s *ConfigurationManager) GetStackPollInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stackPollInterval
}

func (s *ConfigurationManager) GetMaxConcurrentReconciles() *int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxConcurrentReconciles
}

func (s *ConfigurationManager) GetServicePollInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.servicePollInterval
}

func (s *ConfigurationManager) GetBaseRegistryURL() *string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseRegistryURL
}

func (s *ConfigurationManager) SwapBaseRegistry(image string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.baseRegistryURL == nil {
		return image
	}
	if image == "" {
		return image
	}

	parts := strings.SplitN(image, "/", 2)

	// image has a registry (like "registry.plural.sh/nginx:latest")
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		return fmt.Sprintf("%s/%s", s.baseRegistryURL, parts[1])
	}

	// image has no registry (like "nginx:latest")
	return fmt.Sprintf("%s/%s", s.baseRegistryURL, image)
}
