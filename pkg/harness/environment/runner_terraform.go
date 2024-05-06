package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"

	console "github.com/pluralsh/console-client-go"
	"github.com/pluralsh/polly/algorithms"
	"github.com/samber/lo"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

// terraformState targets terraform.tfstate file version 4
type terraformState struct {
	Outputs   terraformOutputs    `json:"outputs"`
	Resources []terraformResource `json:"resources"`
}

type terraformOutputs map[string]terraformOutput

type terraformOutput struct {
	Value     string `json:"value"`
	FieldType string `json:"type"`
	Sensitive bool   `json:"sensitive"`
}

type terraformResource struct {
	Mode      string                      `json:"mode"`
	Type      string                      `json:"type"`
	Name      string                      `json:"name"`
	Provider  string                      `json:"provider"`
	Instances []terraformResourceInstance `json:"instances"`
}

func (in terraformResource) Configuration() string {
	configurationMap := make(map[string]interface{})
	attributesList := algorithms.Map(
		in.Instances,
		func(i terraformResourceInstance) map[string]interface{} {
			return i.Attributes
		},
	)

	for _, attributes := range attributesList {
		maps.Copy(configurationMap, attributes)
	}

	configuration, _ := json.Marshal(configurationMap)
	return string(configuration)
}

func (in terraformResource) Links() []string {
	links := make([]string, 0)

	for _, instance := range in.Instances {
		links = append(links, instance.Dependencies...)
	}

	return links
}

type terraformResourceMode string

const (
	terraformResourceModeManaged terraformResourceMode = "managed"
	terraformResourceModeData    terraformResourceMode = "data"
)

type terraformResourceInstance struct {
	Attributes   map[string]interface{} `json:"attributes"`
	Dependencies []string               `json:"dependencies"`
}

type terraformRunner struct {
	// dir
	dir string

	// planFileName
	planFileName string

	// stateFileName
	stateFileName string
}

func (in *terraformRunner) State() (*console.StackStateAttributes, error) {
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
			func(r terraformResource) *console.StackStateResourceAttributes {
				return in.resource(r)
			}),
	}, nil
}

func (in *terraformRunner) Output() ([]*console.StackOutputAttributes, error) {
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

func (in *terraformRunner) Args(stage console.StepStage) exec.ArgsModifier {
	switch stage {
	case console.StepStagePlan:
		return in.planArgsModifier
	case console.StepStageApply:
		return in.applyArgsModifier
	}

	return nil
}

func (in *terraformRunner) planArgsModifier(args []string) []string {
	if algorithms.Index(args, func(a string) bool {
		return a == "plan"
	}) < 0 {
		return args
	}

	return append(args, fmt.Sprintf("-out=%s", in.planFileName))
}

func (in *terraformRunner) applyArgsModifier(args []string) []string {
	return append(args, fmt.Sprintf(in.planFileName))
}

func (in *terraformRunner) resource(r terraformResource) *console.StackStateResourceAttributes {
	return &console.StackStateResourceAttributes{
		Identifier:    r.Name,
		Resource:      r.Type,
		Name:          r.Name,
		Configuration: lo.ToPtr(r.Configuration()),
		Links:         algorithms.Map(r.Links(), func(d string) *string { return &d }),
	}
}

func (in *terraformRunner) state() (*terraformState, error) {
	state := new(terraformState)
	output, err := os.ReadFile(fmt.Sprintf("%s/%s", in.dir, in.stateFileName))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(output, state)
	if err != nil {
		return nil, err
	}

	return state, nil
}

func (in *terraformRunner) plan() (string, error) {
	output, err := exec.NewExecutable(
		"terraform",
		exec.WithArgs([]string{"show", "-json", in.planFileName}),
		exec.WithDir(in.dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func (in *terraformRunner) init() *terraformRunner {
	// TODO: Allow to override?
	in.stateFileName = "terraform.tfstate"

	// TODO: Allow to override?
	in.planFileName = "terraform.tfplan"
	helpers.EnsureFileOrDie(fmt.Sprintf("%s/%s", in.dir, "terraform.tfplan"))

	return in
}

func newTerraformRunner(dir string) *terraformRunner {
	return (&terraformRunner{
		dir: dir,
	}).init()
}
