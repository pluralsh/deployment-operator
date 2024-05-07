package controller

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	gqlclient "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	internalerrors "github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
	"github.com/pluralsh/deployment-operator/pkg/harness/tool"
)

// Start starts the manager and waits indefinitely.
// There are a couple of ways to have start return:
//   - an error has occurred in one of the internal operations
//   - all commands have finished their execution
//   - it was running for too long and timed out
//   - remote cancellation signal was received and stopped the execution
func (in *stackRunController) Start(ctx context.Context) (retErr error) {
	in.Lock()

	ready := false
	defer func() {
		// Only unlock if we haven't reached
		// the internal readiness condition.
		if !ready {
			in.Unlock()
		}
	}()

	in.preStart()

	if err := in.prepare(); err != nil {
		return err
	}

	// Add executables to executor
	for _, e := range in.executables() {
		if err := in.executor.Add(e); err != nil {
			return err
		}
	}

	if err := in.executor.Start(ctx); err != nil {
		return fmt.Errorf("could not start executor: %w", err)
	}

	ready = true
	in.Unlock()
	select {
	// Stop the execution if provided context is done.
	case <-ctx.Done():
		retErr = context.Cause(ctx)
	// In case of any error finish the execution and return error.
	case err := <-in.errChan:
		retErr = err
	// If execution finished successfully return.
	case <-in.finishedChan:
		retErr = nil
	}

	return in.postStart(retErr)
}

func (in *stackRunController) preStart() {
	if in.stackRun.Status != gqlclient.StackStatusPending && !environment.IsDev() {
		klog.Fatalf("could not start stack run: invalid status: %s", in.stackRun.Status)
	}

	if err := in.markStackRun(gqlclient.StackStatusRunning); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}
}

func (in *stackRunController) postStart(err error) error {
	var status gqlclient.StackStatus

	switch {
	case err == nil:
		status = gqlclient.StackStatusSuccessful
	case errors.Is(err, internalerrors.ErrRemoteCancel):
		status = gqlclient.StackStatusCancelled
	default:
		status = gqlclient.StackStatusFailed
	}

	if err := in.completeStackRun(status, err); err != nil {
		klog.ErrorS(err, "could not complete stack run")
	}

	return err
}

func (in *stackRunController) postStepRun(id string, err error) {
	var status gqlclient.StepStatus

	switch {
	case err == nil:
		status = gqlclient.StepStatusSuccessful
	default:
		status = gqlclient.StepStatusFailed
	}

	if err := in.markStackRunStep(id, status); err != nil {
		klog.ErrorS(err, "could not update stack run step status")
	}
}

func (in *stackRunController) preStepRun(id string) {
	if err := in.markStackRunStep(id, gqlclient.StepStatusRunning); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}
}

func (in *stackRunController) executables() []exec.Executable {
	// Ensure that steps are sorted in the correct order
	slices.SortFunc(in.stackRun.Steps, func(s1, s2 *gqlclient.RunStepFragment) int {
		return cmp.Compare(s1.Index, s2.Index)
	})

	return algorithms.Map(in.stackRun.Steps, func(step *gqlclient.RunStepFragment) exec.Executable {
		consoleWriter := sink.NewConsoleLogWriter(
			in.ctx,
			in.consoleClient,
			append(in.sinkOptions, sink.WithID(step.ID))...,
		)

		return exec.NewExecutable(
			step.Cmd,
			exec.WithDir(in.dir),
			exec.WithEnv(in.stackRun.Env()),
			exec.WithArgs(step.Args),
			exec.WithArgsModifier(in.tool.Modifier(step.Stage).Args),
			exec.WithID(step.ID),
			exec.WithCustomOutputSink(consoleWriter),
			exec.WithCustomErrorSink(consoleWriter),
		)
	})
}

func (in *stackRunController) markStackRun(status gqlclient.StackStatus) error {
	return in.consoleClient.UpdateStackRun(in.stackRunID, gqlclient.StackRunAttributes{
		Status: status,
	})
}

func (in *stackRunController) markStackRunStep(id string, status gqlclient.StepStatus) error {
	return in.consoleClient.UpdateStackRunStep(id, gqlclient.RunStepAttributes{
		Status: status,
	})
}

func (in *stackRunController) completeStackRun(status gqlclient.StackStatus, stackRunErr error) error {
	_, err := in.tool.State()
	if err != nil {
		klog.ErrorS(err, "could not prepare state attributes")
	}

	_, err = in.tool.Output()
	if err != nil {
		klog.ErrorS(err, "could not prepare output attributes")
	}

	serviceErrorAttributes := make([]*gqlclient.ServiceErrorAttributes, 0)
	if stackRunErr != nil {
		serviceErrorAttributes = append(serviceErrorAttributes, &gqlclient.ServiceErrorAttributes{
			Message: stackRunErr.Error(),
		})
	}

	return in.consoleClient.CompleteStackRun(in.stackRunID, gqlclient.StackRunAttributes{
		// TODO: Uncomment once API is fixed.
		//Errors: serviceErrorAttributes,
		//Output: output,
		//State:  state,
		Status: status,
	})
}

func (in *stackRunController) prepare() error {
	env := environment.New(
		environment.WithStackRun(in.stackRun),
		environment.WithWorkingDir(in.dir),
		environment.WithFetchClient(in.fetchClient),
	)

	in.tool = tool.New(in.stackRun.Type, in.dir)

	return env.Setup()
}

func (in *stackRunController) init() (Controller, error) {
	if len(in.stackRunID) == 0 {
		return nil, fmt.Errorf("could not initialize controller: stack run id is empty")
	}

	if in.consoleClient == nil {
		return nil, fmt.Errorf("could not initialize controller: consoleClient is nil")
	}

	if stackRun, err := in.consoleClient.GetStackRun(in.stackRunID); err != nil {
		return nil, err
	} else {
		in.stackRun = stackRun
	}

	return in, nil
}

func NewStackRunController(options ...Option) (Controller, error) {
	finishedChan := make(chan struct{})
	errChan := make(chan error, 1)
	ctrl := &stackRunController{
		errChan:      errChan,
		finishedChan: finishedChan,
		sinkOptions:  make([]sink.Option, 0),
	}

	ctrl.executor = newExecutor(
		errChan,
		finishedChan,
		WithPreRunFunc(ctrl.preStepRun),
		WithPostRunFunc(ctrl.postStepRun),
	)

	for _, option := range options {
		option(ctrl)
	}

	return ctrl.init()
}
