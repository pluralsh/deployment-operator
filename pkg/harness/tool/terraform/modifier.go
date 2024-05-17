package terraform

import (
	"fmt"

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
	if !helpers.IsExists(in.planFileName) {
		return args
	}

	return append(args, fmt.Sprintf(in.planFileName))
}

func NewApplyModifier(planFileName string) *ApplyModifier {
	return &ApplyModifier{planFileName}
}
