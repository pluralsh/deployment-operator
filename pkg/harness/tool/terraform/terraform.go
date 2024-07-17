package terraform

import (
	"context"
	"fmt"
	"path"

	tfjson "github.com/hashicorp/terraform-json"
	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	tfapi "github.com/pluralsh/deployment-operator/pkg/harness/tool/terraform/api"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// State implements [v1.Tool] interface.
func (in *Terraform) State() (*console.StackStateAttributes, error) {
	state, err := in.state()
	if err != nil || state.Values == nil || state.Values.RootModule == nil {
		return nil, err
	}

	resources := make([]*console.StackStateResourceAttributes, 0)
	_ = algorithms.BFS(state.Values.RootModule, func(module *tfjson.StateModule) ([]*tfjson.StateModule, error) {
		return module.ChildModules, nil
	}, func(module *tfjson.StateModule) error {
		klog.V(log.LogLevelTrace).InfoS("visiting module", "module", module)
		resources = append(
			resources,
			tfapi.ToStackStateResourceAttributesList(state.Values.RootModule.Resources)...,
		)

		return nil
	})

	return &console.StackStateAttributes{
		State: resources,
	}, nil
}

// Plan implements [v1.Tool] interface.
func (in *Terraform) Plan() (*console.StackStateAttributes, error) {
	plan, err := in.plan()
	if err != nil {
		return nil, err
	}

	return &console.StackStateAttributes{
		Plan: &plan,
	}, nil
}

// Output implements [v1.Tool] interface.
func (in *Terraform) Output() ([]*console.StackOutputAttributes, error) {
	result := make([]*console.StackOutputAttributes, 0)

	state, err := in.state()
	if err != nil || state.Values == nil || state.Values.Outputs == nil {
		return nil, err
	}

	for k, v := range state.Values.Outputs {
		result = append(result, &console.StackOutputAttributes{
			Name:   k,
			Value:  tfapi.OutputValueString(v.Value),
			Secret: lo.ToPtr(v.Sensitive),
		})
	}

	return result, nil
}

// Modifier implements [v1.Tool] interface.
func (in *Terraform) Modifier(stage console.StepStage) v1.Modifier {
	switch stage {
	case console.StepStagePlan:
		return NewPlanArgsModifier(in.planFileName)
	case console.StepStageApply:
		return NewApplyArgsModifier(in.dir, in.planFileName)
	}

	return v1.NewDefaultModifier()
}

// ConfigureStateBackend implements [v1.Tool] interface.
func (in *Terraform) ConfigureStateBackend(actor, deployToken string, urls *console.StackRunBaseFragment_StateUrls) error {
	input := &OverrideTemplateInput{
		Address:       lo.FromPtr(urls.Terraform.Address),
		LockAddress:   lo.FromPtr(urls.Terraform.Lock),
		UnlockAddress: lo.FromPtr(urls.Terraform.Unlock),
		Actor:         actor,
		DeployToken:   deployToken,
	}
	fileName, content, err := overrideTemplate(input)
	if err != nil {
		return err
	}

	if err = helpers.File().Create(path.Join(in.dir, fileName), content); err != nil {
		return fmt.Errorf("failed configuring state backend file %q: %w", fileName, err)
	}

	return nil
}

func (in *Terraform) state() (*tfjson.State, error) {
	state := new(tfjson.State)
	output, err := exec.NewExecutable(
		"terraform",
		exec.WithArgs([]string{"show", "-json"}),
		exec.WithDir(in.dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return state, err
	}

	err = state.UnmarshalJSON(output)
	if err != nil {
		return nil, err
	}

	klog.V(log.LogLevelTrace).InfoS("terraform state read successfully", "state", state)
	return state, nil
}

func (in *Terraform) plan() (string, error) {
	output, err := exec.NewExecutable(
		"terraform",
		exec.WithArgs([]string{"show", in.planFileName}),
		exec.WithDir(in.dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return "", err
	}

	klog.V(log.LogLevelTrace).InfoS("terraform plan file read successfully", "file", in.planFileName, "output", string(output))
	return string(output), nil
}

func (in *Terraform) init() *Terraform {
	in.planFileName = "terraform.tfplan"
	helpers.EnsureFileOrDie(path.Join(in.dir, in.planFileName))

	return in
}

// New creates a Terraform structure that implements v1.Tool interface.
func New(dir string) v1.Tool {
	return (&Terraform{dir: dir}).init()
}
