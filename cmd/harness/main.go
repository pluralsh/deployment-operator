package main

import (
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/harness/args"
	"github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
)

func main() {
	consoleClient := client.New(args.ConsoleUrl(), args.ConsoleToken())
	// TODO: Should retry?
	stackRun, err := consoleClient.GetStackRun(args.StackRunID())
	if err != nil {
		klog.Fatal(err)
	}

	env := environment.New(stackRun)
	err = env.Prepare()
	if err != nil {
		klog.Fatal(err)
	}
}
