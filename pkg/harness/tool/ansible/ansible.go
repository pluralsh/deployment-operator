package ansible

import (
	"os"
	"path"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *Ansible) Plan() (*console.StackStateAttributes, error) {
	output, err := os.ReadFile(in.planFilePath)
	if err != nil {
		return nil, err
	}

	klog.V(log.LogLevelTrace).InfoS("ansible plan file read successfully", "file", in.planFilePath, "output", string(output))
	return &console.StackStateAttributes{
		Plan: lo.ToPtr(string(output)),
	}, nil
}

// State is not supported by ansible.
func (in *Ansible) State() (*console.StackStateAttributes, error) {
	return nil, nil
}

// Output is not supported by ansible.
func (in *Ansible) Output() ([]*console.StackOutputAttributes, error) {
	// TODO: add logic
	return []*console.StackOutputAttributes{}, nil
}

// Modifier is not required by ansible.
func (in *Ansible) Modifier(stage console.StepStage) v1.Modifier {
	if stage == console.StepStagePlan {
		return NewPlanModifier(in.planFilePath)
	}

	// TODO: add logic
	return v1.NewProxyModifier()
}

// ConfigureStateBackend is not supported by ansible.
func (in *Ansible) ConfigureStateBackend(_, _ string, _ *console.StackRunBaseFragment_StateUrls) error {
	return nil
}

func (in *Ansible) init() *Ansible {
	in.planFileName = "ansible.plan"
	in.planFilePath = path.Join(in.dir, in.planFileName)
	helpers.EnsureFileOrDie(in.planFilePath)

	return in
}

func New(dir string) *Ansible {
	return (&Ansible{dir: dir}).init()
}
