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
)

func main() {
	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())
	fetchClient := helpers.Fetch(
		helpers.FetchWithToken(args.ConsoleToken()),
		helpers.FetchToDir(args.WorkingDir()),
	)
	ctx := signals.NewCancelableContext(
		signals.SetupSignalHandler(signals.ExitCodeOther),
		signals.NewTimeoutSignal(args.Timeout()),
		signals.NewConsoleSignal(consoleClient),
	)

	ctrl, err := controller.NewStackRunController(
		controller.WithStackRun(args.StackRunID()),
		controller.WithConsoleClient(consoleClient),
		controller.WithFetchClient(fetchClient),
		controller.WithWorkingDir(args.WorkingDir()),
	)
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
		klog.ErrorS(err, "timed out waiting for stack run to complete", "timeout", args.Timeout())
		os.Exit(signals.ExitCodeTimeout.Int())
	case errors.Is(err, internalerrors.ErrRemoteCancel):
		klog.ErrorS(err, "stack run has been cancelled")
		os.Exit(signals.ExitCodeCancel.Int())
	}

	klog.ErrorS(err, "stack run failed")
	os.Exit(signals.ExitCodeOther.Int())
}
