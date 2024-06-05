package ansible

import (
	console "github.com/pluralsh/console-client-go"

	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

func (in *Ansible) Plan() (*console.StackStateAttributes, error) {
	// TODO implement me
	panic("implement me")
}

func (in *Ansible) State() (*console.StackStateAttributes, error) {
	// TODO implement me
	panic("implement me")
}

func (in *Ansible) Output() ([]*console.StackOutputAttributes, error) {
	// TODO implement me
	panic("implement me")
}

func (in *Ansible) Modifier(stage console.StepStage) v1.Modifier {
	// TODO implement me
	panic("implement me")
}

func (in *Ansible) ConfigureStateBackend(actor, deployToken string, urls *console.StackRunBaseFragment_StateUrls) error {
	// TODO implement me
	panic("implement me")
}

func New(dir string) *Ansible {
	return &Ansible{dir: dir}
}
