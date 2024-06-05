package controller

import (
	"context"
	"sync"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
	stackrunv1 "github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
	toolv1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

type Controller interface {
	Start(ctx context.Context) error
	Finish(stackRunErr error) error
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
	stackRun *stackrunv1.StackRun

	// consoleClient
	consoleClient console.Client

	// consoleToken
	consoleToken string

	// fetchClient
	fetchClient helpers.FetchClient

	// dir
	dir string

	// execOptions
	execOptions []exec.Option

	// sinkOptions allows providing custom options to
	// sink.ConsoleWriter. By default, every command output
	// is being forwarded both to the os.Stdout and sink.ConsoleWriter.
	sinkOptions []sink.Option

	// tool handles one of the supported infrastructure management tools.
	// List of supported tools is based on the gqlclient.StackType.
	// It is mainly responsible for:
	// - gathering state
	tool toolv1.Tool

	// wg
	wg sync.WaitGroup

	// stopChan
	stopChan chan struct{}
}

type Option func(*stackRunController)
