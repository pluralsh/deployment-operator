package main

import (
	"errors"
	"os"

	"github.com/pluralsh/deployment-operator/pkg/harness/signals"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/sentinel-harness/args"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	internalerrors "github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/pluralsh/deployment-operator/pkg/sentinel-harness/controller"
)

func main() {
	klog.V(log.LogLevelDefault).InfoS("starting sentinel harness", "version", environment.Version)

	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())

	ctx := signals.NewCancelableContext(
		signals.SetupSignalHandler(signals.ExitCodeTerminated),
		signals.NewConsoleSignal(consoleClient, args.SentinelRunID()),
	)

	opts := []controller.Option{
		controller.WithSentinelRun(args.SentinelRunID()),
		controller.WithConsoleClient(consoleClient),
		controller.WithConsoleToken(args.ConsoleToken()),
		controller.WithWorkingDir(args.WorkingDir()),
	}

	ctrl, err := controller.NewSentinelRunController(opts...)
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
		klog.ErrorS(err, "timed out waiting for stack run step to complete", "timeout", args.Timeout())
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
