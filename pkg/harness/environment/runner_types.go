package environment

import (
	console "github.com/pluralsh/console-client-go"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type runner interface {
	State() (*console.StackStateAttributes, error)
	Output() ([]*console.StackOutputAttributes, error)
	Args(stage console.StepStage) exec.ArgsModifier
}
