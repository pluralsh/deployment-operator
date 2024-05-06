package environment

import (
	console "github.com/pluralsh/console-client-go"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type ansibleRunner struct {
	// dir
	dir string
}

func (in *ansibleRunner) State() (*console.StackStateAttributes, error) {
	//TODO implement me
	panic("implement me")
}

func (in *ansibleRunner) Output() ([]*console.StackOutputAttributes, error) {
	//TODO implement me
	panic("implement me")
}

func (in *ansibleRunner) Args(stage console.StepStage) exec.ArgsModifier {
	//TODO implement me
	panic("implement me")
}

func newAnsibleRunner(dir string) *ansibleRunner {
	return &ansibleRunner{dir}
}
