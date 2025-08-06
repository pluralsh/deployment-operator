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

	if err := setDuration(config.ClusterPingInterval, s.clusterPingInterval); err != nil {
		return err
	}
	if err := setDuration(config.RuntimeServicesPingInterval, s.runtimeServicesPingInterval); err != nil {
		return err
	}
	if err := setDuration(config.PipelineGateInterval, s.pipelineGateInterval); err != nil {
		return err
	}
	if err := setDuration(config.StackPollInterval, s.stackPollInterval); err != nil {
		return err
	}
	if err := setDuration(config.VulnerabilityReportUploadInterval, s.vulnerabilityReportUploadInterval); err != nil {
		return err
	}
	s.maxConcurrentReconciles = config.MaxConcurrentReconciles

	return nil
}

func setDuration(interval *string, d *time.Duration) error {
	if interval == nil {
		return nil
	}
	duration, err := time.ParseDuration(*interval)
	if err != nil {
		return err
	}
	d = &duration
	return nil
}

func (s *ConfigurationManager) GetClusterPingInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clusterPingInterval
}

func (s *ConfigurationManager) RuntimeServicesPingInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeServicesPingInterval
}

func (s *ConfigurationManager) GetVulnerabilityReportUploadInterval() *time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vulnerabilityReportUploadInterval
}
