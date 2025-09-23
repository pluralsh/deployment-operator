package environment

import (
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
)

// Environment is responsible for handling agent run working directory.
// It can initialize directories and clone repositories required by the agent run.
type Environment interface {
	// Setup ensures that the environment is correctly initialized
	// in order to start the agent run.
	//
	// 1. Creates a working dir if it doesn't exist.
	// 2. Clones the target repository using SCM credentials.
	Setup() error
}

type environment struct {
	// agentRun provides all information required to prepare
	// the environment and working directory for the actual
	// execution of the agent run, including repository URL
	// and SCM credentials for cloning.
	agentRun *v1.AgentRun
	// dir is the working directory where the repository will be cloned.
	dir string
}

// Option allows to modify Environment behavior.
type Option func(*environment)
