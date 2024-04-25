package controller

import (
	"sync"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type executor struct {
	startQueue []exec.Executable
	start sync.Mutex
	started bool

	// errChan is the error channel passed by the caller
	// when the executor is created.
	// All errors are forwarded to this channel once they occur.
	errChan chan error

	// finishedChan is a channel passed by the caller
	// when the executor is created.
	// Once all commands are executed without any error
	// it will be closed to signal successful completion.
	finishedChan chan struct{}

	// preRunFunc is executed before every command.
	preRunFunc func(id string)

	// postRunFunc is executed after every command.
	// It provides information about execution status: nil or error.
	postRunFunc func(id string, err error)

	// strategy
	strategy ExecutionStrategy

	// ch is the internal channel where the executables are read off from.
	// It is used only by the ExecutionStrategyOrdered to ensure ordered
	// run of the executables.
	ch chan exec.Executable
}

type ExecutorOption func(*executor)

type ExecutionStrategy string

const (
	ExecutionStrategyParallel ExecutionStrategy = "parallel"
	ExecutionStrategyOrdered  ExecutionStrategy = "ordered"
)
