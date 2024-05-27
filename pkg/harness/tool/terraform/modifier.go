package terraform

import (
	"fmt"
	"path"

	"github.com/pluralsh/polly/algorithms"

	"github.com/pluralsh/deployment-operator/internal/helpers"
)

// Args implements exec.ArgsModifier type.
func (in *InitModifier) Args(args []string) []string {
	return args
}

func NewInitModifier() *InitModifier {
	return &InitModifier{}
}

// Args implements exec.ArgsModifier type.
func (in *PlanModifier) Args(args []string) []string {
	if algorithms.Index(args, func(a string) bool {
		return a == "plan"
	}) < 0 {
		return args
	}

	return append(args, fmt.Sprintf("-out=%s", in.planFileName))
}

func NewPlanModifier(planFileName string) *PlanModifier {
	return &PlanModifier{planFileName}
}

func (in *ApplyModifier) Args(args []string) []string {
	if !helpers.IsExists(path.Join(in.dir, in.planFileName)) ||
		// This is to avoid doubling plan file arg if API already adds it
		algorithms.Index(args, func(a string) bool {
			return a == in.planFileName
		}) >= 0 {
		return args
	}

	return append(args, in.planFileName)
}

func NewApplyModifier(dir, planFileName string) *ApplyModifier {
	return &ApplyModifier{planFileName, dir}
}
