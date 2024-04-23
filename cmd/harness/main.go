package main

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/harness/args"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

func main() {
	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())
	// TODO: Should retry?
	stackRun, err := consoleClient.GetStackRun(args.StackRunID())
	if err != nil {
		klog.Fatal(err)
	}

	env := environment.New(
		stackRun,
		environment.WithWorkingDir(args.WorkingDir()),
		environment.WithFetchClient(helpers.Fetch(
			helpers.FetchWithToken(args.ConsoleToken()),
		)),
	)
	err = env.Prepare()
	if err != nil {
		klog.Fatal(err)
	}

	// TODO: Catch stop/kill signal and attempt to do a proper cleanup/logs flush
	cmd := exec.NewExecutable(
		"sleep",
		exec.WithDir(env.WorkingDir()),
		exec.WithCancelableContext(
			context.Background(),
			exec.NewTimeoutSignal(args.Timeout()),
		),
	)
	err = cmd.Run("5")
	if err != nil {
		klog.Fatal(err)
	}
}
