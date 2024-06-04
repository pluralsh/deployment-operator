package ansible

import (
	"context"
	"os"

	console "github.com/pluralsh/console-client-go"
	"k8s.io/klog"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

func (in *Ansible) Plan() (string, error) {
	playbook := os.Getenv("PLAYBOOK")
	output, err := exec.NewExecutable(
		"ansible-playbook",
		exec.WithArgs([]string{playbook, "--check", "--diff"}),
		exec.WithDir(in.dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return "", err
	}

	klog.V(5).Infoln("Ansible Check was Successful", "output", string(output))
	return string(output), nil
}

func (in *Ansible) State() (*console.StackStateAttributes, error) {
	// Ansible doesn't have a state
	return nil, nil
}

func (in *Ansible) Output() ([]*console.StackOutputAttributes, error) {
	// TODO implement me
	// 1. Run ansible-playbook --diff
	// 2. Parse the output and return the state
	panic("implement me")
}

func (in *Ansible) Modifier(stage console.StepStage) v1.Modifier {
	// TODO implement me
	panic("implement me")
}

func (in *Ansible) ConfigureStateBackend(actor, deployToken string, urls *console.StackRunBaseFragment_StateUrls) error {
	//Ansible doesn't have a state backend
	return nil
}

func New(dir string) *Ansible {
	return &Ansible{dir: dir}
}
