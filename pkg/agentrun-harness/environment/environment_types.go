package environment

import (
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
)

// environment implements Environment interface.
type environment struct {
	// agentRun provides all information required to prepare
	// the environment and working directory for the actual
	// execution of the agent run, including repository URL
	// and SCM credentials for cloning.
	agentRun *v1.AgentRun
	// dir is the working directory where the repository will be cloned.
	dir string
}

// Option allows modifying Environment behavior.
type Option func(*environment)
