package controller

import (
	"context"
	"sync"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness"
)

type Controller interface {
	Start(ctx context.Context) error
}

type stackRunController struct {
	sync.Mutex

	internalCtx    context.Context
	internalCancel context.CancelFunc

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
}

type Informer interface {
}

type terraformInformer struct {
}

type ansibleInformer struct {
}

type Option func(*stackRunController)
