package controller

import (
	"context"
	"sync"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

type Controller interface {
	Start(ctx context.Context) error
}

type agentRunController struct {
	sync.Mutex

	// wg is used to wait for all commands to finish
	wg sync.WaitGroup

	// agentRun is the agent run that is being processed
	agentRun *agentrunv1.AgentRun

	// agentRunID is the ID of the agent run that is being processed
	agentRunID string

	// consoleClient is the client for Console API
	consoleClient console.Client

	// consoleToken is the token used to access the Console API
	consoleToken string

	// dir is the working directory where the repository will be cloned.
	dir string

	// executor is the executor that will run the commands
	executor exec.Executor

	// execOptions allows providing custom options to exec.Executable.
	execOptions []exec.Option

	// sinkOptions allows providing custom options to
	// sink.ConsoleWriter. By default, every command output
	// is being forwarded both to the os.Stdout and sink.ConsoleWriter.
	sinkOptions []sink.Option

	// errChan is the error channel passed by the caller
	errChan chan error

	// finishedChan is a channel that is closed when the controller has finished processing
	finishedChan chan struct{}

	// stopChan
	stopChan chan struct{}
}

type Option func(*agentRunController)
