package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	v4 "github.com/pluralsh/deployment-operator/pkg/harness/tool/terraform/api/v4"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// State implements v1.Tool interface.
func (in *Terraform) State() (*console.StackStateAttributes, error) {
	plan, err := in.plan()
	if err != nil {
		return nil, err
	}

	state, err := in.state()
	if err != nil {
		return nil, err
	}

	return &console.StackStateAttributes{
		Plan: &plan,
		State: algorithms.Map(
			state.Resources,
			func(r v4.Resource) *console.StackStateResourceAttributes {
				return in.resource(r)
			}),
	}, nil
}

// Output implements v1.Tool interface.
func (in *Terraform) Output() ([]*console.StackOutputAttributes, error) {
	result := make([]*console.StackOutputAttributes, 0)

	state, err := in.state()
	if err != nil {
		return nil, err
	}

	for k, v := range state.Outputs {
		result = append(result, &console.StackOutputAttributes{
			Name:   k,
			Value:  v.Value,
			Secret: &v.Sensitive,
		})
	}

	return result, nil
}

// Modifier implements v1.Tool interface.
func (in *Terraform) Modifier(stage console.StepStage) v1.Modifier {
	switch stage {
	case console.StepStageInit:
		return NewInitModifier()
	case console.StepStagePlan:
		return NewPlanModifier(in.planFileName)
	case console.StepStageApply:
		return NewApplyModifier(in.planFileName)
	}

	return v1.NewProxyModifier()
}

func (in *Terraform) resource(r v4.Resource) *console.StackStateResourceAttributes {
	return &console.StackStateResourceAttributes{
		Identifier:    r.Name,
		Resource:      r.Type,
		Name:          r.Name,
		Configuration: lo.ToPtr(r.Configuration()),
		Links:         algorithms.Map(r.Links(), func(d string) *string { return &d }),
	}
}

func (in *Terraform) state() (*v4.State, error) {
	state := new(v4.State)
	output, err := os.ReadFile(fmt.Sprintf("%s/%s", in.dir, in.stateFileName))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(output, state)
	if err != nil {
		return nil, err
	}

	klog.V(log.LogLevelDebug).InfoS("terraform state file parsed successfully", "file", in.stateFileName, "state", state)
	return state, nil
}

func (in *Terraform) plan() (string, error) {
	output, err := exec.NewExecutable(
		"terraform",
		exec.WithArgs([]string{"show", "-json", in.planFileName}),
		exec.WithDir(in.dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return "", err
	}

	klog.V(log.LogLevelDebug).InfoS("terraform plan file read successfully", "file", in.planFileName, "output", string(output))
	return string(output), nil
}

func (in *Terraform) init() *Terraform {
	// TODO: Allow to override?
	in.stateFileName = "terraform.tfstate"

	// TODO: Allow to override?
	in.planFileName = "terraform.tfplan"
	helpers.EnsureFileOrDie(fmt.Sprintf("%s/%s", in.dir, "terraform.tfplan"))

	return in
}

// New creates a Terraform structure that implements v1.Tool interface.
func New(dir string) *Terraform {
	return (&Terraform{dir: dir}).init()
}
