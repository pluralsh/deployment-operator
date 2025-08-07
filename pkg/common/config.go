package common

import (
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
	mu                                sync.RWMutex
	clusterPingInterval               *time.Duration
	runtimeServicesPingInterval       *time.Duration
	stackPollInterval                 *time.Duration
	vulnerabilityReportUploadInterval *time.Duration
	pipelineGateInterval              *time.Duration
	maxConcurrentReconciles           *int
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

	interval, err = setDuration(config.RuntimeServicesPingInterval)
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
	s.vulnerabilityReportUploadInterval = interval
	s.maxConcurrentReconciles = config.MaxConcurrentReconciles

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

func (s *ConfigurationManager) GetVulnerabilityReportUploadInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vulnerabilityReportUploadInterval
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
