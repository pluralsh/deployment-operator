package controller

import (
	"context"
	"sync"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/exec"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

type Controller interface {
	Start(ctx context.Context) error
}

type agentRunController struct {
	sync.Mutex

	// executor
	executor *executor

	agentRun *agentrunv1.AgentRun

	// consoleClient
	consoleClient console.Client

	// fetchClient
	fetchClient helpers.FetchClient

	// execOptions
	execOptions []exec.Option

	// sinkOptions allows providing custom options to
	// sink.ConsoleWriter. By default, every command output
	// is being forwarded both to the os.Stdout and sink.ConsoleWriter.
	sinkOptions []sink.Option

	agentRunID string

	// consoleToken
	consoleToken string

	// dir
	dir string

	// wg
	wg sync.WaitGroup

	// errChan
	errChan chan error

	// finishedChan
	finishedChan chan struct{}

	// stopChan
	stopChan chan struct{}
}

type Option func(*agentRunController)
