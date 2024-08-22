package ansible

import (
	"os"
	"path"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// Plan implements [v1.Tool] interface.
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

// Modifier implements [v1.Tool] interface.
func (in *Ansible) Modifier(stage console.StepStage) v1.Modifier {
	modifiers := []v1.Modifier{NewGlobalEnvModifier(in.workDir)}

	if in.variables != nil {
		modifiers = append(modifiers, NewVariableInjectorModifier(in.variablesFileName))
	}

	if stage == console.StepStagePlan {
		modifiers = append(modifiers, NewPassthroughModifier(in.planFilePath))
	}

	return v1.NewMultiModifier(modifiers...)
}

func (in *Ansible) init() *Ansible {
	in.planFileName = "ansible.plan"
	in.planFilePath = path.Join(in.execDir, in.planFileName)
	helpers.EnsureFileOrDie(in.planFilePath, nil)

	in.variablesFileName = "plural.variables.json"
	helpers.EnsureFileOrDie(path.Join(in.execDir, in.variablesFileName), in.variables)

	return in
}

// New creates an Ansible structure that implements v1.Tool interface.
func New(workDir, execDir string, variables *string) v1.Tool {
	return (&Ansible{workDir: workDir, execDir: execDir, variables: variables}).init()
}
