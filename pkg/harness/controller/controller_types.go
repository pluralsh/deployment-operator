package controller

import (
	"context"
	"sync"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

type Controller interface {
	Start(ctx context.Context) error
}

type stackRunController struct {
	sync.Mutex

	// errChan
	errChan chan error

	// finishedChan
	finishedChan chan struct{}

	// executor
	executor *executor

	// stackRunID
	stackRunID string

	// stackRun
	stackRun *harness.StackRun

	// consoleClient
	consoleClient console.Client

	// fetchClient
	fetchClient helpers.FetchClient

	// dir
	dir string

	// sinkOptions allows providing custom options to
	// sink.ConsoleWriter. By default, every command output
	// is being forwarded both to the os.Stdout and sink.ConsoleWriter.
	sinkOptions []sink.Option
}

type Option func(*stackRunController)
