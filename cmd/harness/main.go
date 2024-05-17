package main

import (
	"errors"
	"os"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/harness/args"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/controller"
	internalerrors "github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/harness/signals"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

func main() {
	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())
	fetchClient := helpers.Fetch(
		helpers.FetchWithToken(args.ConsoleToken()),
		helpers.FetchToDir(args.WorkingDir()),
	)
	ctx := signals.NewCancelableContext(
		signals.SetupSignalHandler(signals.ExitCodeTerminated),
		signals.NewTimeoutSignal(args.Timeout()),
		signals.NewConsoleSignal(consoleClient, args.StackRunID()),
	)

	ctrl, err := controller.NewStackRunController(
		controller.WithStackRun(args.StackRunID()),
		controller.WithConsoleClient(consoleClient),
		controller.WithFetchClient(fetchClient),
		controller.WithWorkingDir(args.WorkingDir()),
		controller.WithSinkOptions(
			sink.WithThrottle(args.LogFlushFrequency()),
			sink.WithBufferSizeLimit(args.LogFlushBufferSize()),
		),
	)
	if err != nil {
		handleFatalError(err)
	}

	if err = ctrl.Start(ctx); err != nil {
		handleFatalError(err)
	}
}

func handleFatalError(err error) {
	// TODO: initiate a graceful shutdown procedure

	switch {
	case errors.Is(err, internalerrors.ErrTimeout):
		klog.ErrorS(err, "timed out waiting for stack run to complete", "timeout", args.Timeout())
		os.Exit(signals.ExitCodeTimeout.Int())
	case errors.Is(err, internalerrors.ErrRemoteCancel):
		klog.ErrorS(err, "stack run has been cancelled")
		os.Exit(signals.ExitCodeCancel.Int())
	case errors.Is(err, internalerrors.ErrTerminated):
		klog.ErrorS(err, "stack run has been terminated")
		os.Exit(signals.ExitCodeTerminated.Int())
	}

	klog.ErrorS(err, "stack run failed")
	os.Exit(signals.ExitCodeOther.Int())
}
