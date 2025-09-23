package main

import (
	"errors"
	"os"

	"github.com/pluralsh/deployment-operator/cmd/agent-harness/args"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/controller"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/client"
	internalerrors "github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/harness/signals"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
	"k8s.io/klog/v2"
)

func main() {
	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())
	fetchClient := helpers.Fetch(
		helpers.FetchWithToken(args.ConsoleToken()),
		helpers.FetchToDir(args.WorkingDir()),
	)

	ctx := signals.NewCancelableContext(
		signals.SetupSignalHandler(signals.ExitCodeTerminated),
		signals.NewConsoleSignal(consoleClient, args.AgentRunID()),
	)

	opts := []controller.Option{
		controller.WithAgentRun(args.AgentRunID()),
		controller.WithConsoleClient(consoleClient),
		controller.WithConsoleToken(args.ConsoleToken()),
		controller.WithFetchClient(fetchClient),
		controller.WithWorkingDir(args.WorkingDir()),
		controller.WithSinkOptions(
			sink.WithThrottle(args.LogFlushFrequency()),
			sink.WithBufferSizeLimit(args.LogFlushBufferSize()),
		),
		controller.WithExecOptions(
			exec.WithTimeout(args.Timeout()),
		),
	}

	ctrl, err := controller.NewAgentRunController(opts...)
	if err != nil {
		handleFatalError(err)
	}

	if err = ctrl.Start(ctx); err != nil {
		handleFatalError(err)
	}

}

func handleFatalError(err error) {
	switch {
	case errors.Is(err, internalerrors.ErrTimeout):
		klog.ErrorS(err, "timed out waiting for agent run to complete", "timeout", args.Timeout())
		os.Exit(signals.ExitCodeTimeout.Int())
	case errors.Is(err, internalerrors.ErrRemoteCancel):
		klog.ErrorS(err, "agent run has been cancelled")
		os.Exit(signals.ExitCodeCancel.Int())
	case errors.Is(err, internalerrors.ErrTerminated):
		klog.ErrorS(err, "agent run has been terminated")
		os.Exit(signals.ExitCodeTerminated.Int())
	}

	klog.ErrorS(err, "agent run failed")
	os.Exit(signals.ExitCodeOther.Int())
}
