package controller

import (
	"context"
	"fmt"

	gqlclient "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"

	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

// Start takes care of few things:
// 1. Marks gqlclient.StackRun status as gqlclient.StackStatusRunning
// 2. Prepares all commands and ensures env vars are passed to them
// 3. Executes gqlclient.RunStepFragment commands and marks their status accordingly
// 4. During the execution forwards command output the Console
// 5. Once all steps finished or run failed updates gqlclient.StackRun status accordingly

// Start starts the manager and waits indefinitely.
// There are a couple of ways to have start return:
//   - an error has occurred in one of the internal operations
//   - all commands have finished their execution
//   - it was running for too long and timed out
//   - remote cancellation signal was received and stopped the execution
func (in *stackRunController) Start(ctx context.Context) (err error) {
	in.Lock()

	ready := false
	defer func() {
		// Only unlock if we haven't reached
		// the internal readiness condition.
		if !ready {
			in.Unlock()
		}
	}()

	// Add executables to executor
	for _, e := range in.executables() {
		if err = in.executor.Add(e); err != nil {
			return err
		}
	}

	if err = in.executor.Start(ctx); err != nil {
		return fmt.Errorf("could not start executor: %w", err)
	}

	ready = true
	in.Unlock()
	select {
	// Stop the execution if provided context is done.
	case <-ctx.Done():
		return context.Cause(ctx)
	// In case of any error finish the execution and return error.
	case err = <-in.errChan:
		return err
	// If execution finished successfully return.
	case <-in.finishedChan:
		return nil
	}
}

func (in *stackRunController) executables() []exec.Executable {
	return algorithms.Map(in.stackRun.Steps, func(step *gqlclient.RunStepFragment) exec.Executable {
		return exec.NewExecutable(
			step.Cmd,
			exec.WithDir(in.dir),
			exec.WithEnv(in.stackRun.Env()),
			exec.WithArgs([]string{"asd"}),
			//exec.WithArgs(step.Args),
		)
	})
}

//func (in *stackRunController) markStackRun(status gqlclient.StackStatus) error {
//	return in.consoleClient.UpdateStackRun(in.stackRun().ID, gqlclient.StackRunAttributes{
//		Status: status,
//	})
//}
//
//func (in *stackRunController) markStackRunStep(status gqlclient.StepStatus) error {
//	return in.consoleClient.UpdateStackRunStep(in.stackRun().ID, gqlclient.RunStepAttributes{
//		Status: status,
//	})
//}

func (in *stackRunController) prepare() error {
	env := environment.New(
		environment.WithStackRun(in.stackRun),
		environment.WithWorkingDir(in.dir),
		environment.WithFetchClient(in.fetchClient),
	)

	return env.Setup()
}

func (in *stackRunController) init() (Controller, error) {
	if len(in.stackRunID) == 0 {
		return nil, fmt.Errorf("could not initialize controller: stack run id is empty")
	}

	if in.consoleClient == nil {
		return nil, fmt.Errorf("could not initialize controller: consoleClient is nil")
	}

	// TODO: should retry?
	if stackRun, err := in.consoleClient.GetStackRun(in.stackRunID); err != nil {
		return nil, err
	} else {
		in.stackRun = stackRun
	}

	return in, in.prepare()
}

func NewStackRunController(options ...Option) (Controller, error) {
	finishedChan := make(chan struct{})
	errChan := make(chan error, 1)
	runner := &stackRunController{
		errChan:  errChan,
		finishedChan: finishedChan,
		executor: newExecutor(errChan, finishedChan),
	}

	for _, option := range options {
		option(runner)
	}

	return runner.init()
}
