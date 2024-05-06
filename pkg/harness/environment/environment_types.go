package environment

import (
	console "github.com/pluralsh/console-client-go"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type Environment interface {
	Setup() error
	State() (*console.StackStateAttributes, error)
	Output() ([]*console.StackOutputAttributes, error)
	Args(stage console.StepStage) exec.ArgsModifier
}

type environment struct {
	// stackRun ...
	// TODO: doc
	stackRun *harness.StackRun
	// dir ...
	// TODO: doc
	dir string
	// fetchClient
	// TODO: doc
	fetchClient helpers.FetchClient
	// runner
	// TODO: doc
	runner runner
}

type Option func(*environment)
