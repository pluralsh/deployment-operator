package controller

import (
	"sync"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
)

type executor struct {
	startQueue []exec.Executable
	start      sync.Mutex
	started    bool

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

	// strategy defines how executables queue should be processed internally.
	// ExecutionStrategyParallel - will run all executables at the same time
	//							   without waiting for the previous one to finish
	// 							   its execution.
	// ExecutionStrategyOrdered  - will run executables one by one and wait
	// 							   for the previous one to finish first.
	strategy ExecutionStrategy

	// ch is the internal channel where the executables are read off from.
	// It is used only by the ExecutionStrategyOrdered to ensure ordered
	// run of the executables.
	ch chan exec.Executable

	// hookFunctions ...
	hookFunctions map[stackrun.Lifecycle]stackrun.HookFunction
}

type ExecutorOption func(*executor)

// ExecutionStrategy defines how executables queue should be processed.
type ExecutionStrategy string

const (
	ExecutionStrategyParallel ExecutionStrategy = "parallel"
	ExecutionStrategyOrdered  ExecutionStrategy = "ordered"
)
