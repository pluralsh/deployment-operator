package controller

import (
	"context"
	"sync"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	console "github.com/pluralsh/deployment-operator/pkg/client"
)

type Controller interface {
	Start(ctx context.Context) error
}

type agentRunController struct {
	sync.Mutex

	// agentRun is the agent run that is being processed
	agentRun *agentrunv1.AgentRun

	// agentRunID is the ID of the agent run that is being processed
	agentRunID string

	// consoleClient is the client for Console API
	consoleClient console.Client

	// deployToken is the token used to access the Console External API
	deployToken string

	// consoleURl is needed for MCP Server
	consoleUrl string

	// tool is the agent run tool that is being executed
	tool v1.Tool

	// dir is the working directory where the repository will be cloned.
	dir string

	// errChan signals that an error occurred during command execution
	errChan chan error

	// done signals that all commands execution is finished
	done chan struct{}
}

type Option func(*agentRunController)
