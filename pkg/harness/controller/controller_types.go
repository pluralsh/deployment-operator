package controller

import (
	"context"
	"sync"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

type Controller interface {
	Start(ctx context.Context) error
}

type stackRunController struct {
	ctx context.Context

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
	stackRun *stackrun.StackRun

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

	// tool handles one of the supported infrastructure management tools.
	// List of supported tools is based on the gqlclient.StackType.
	// It is mainly responsible for:
	// - gathering state
	tool v1.Tool
}

type Option func(*stackRunController)
