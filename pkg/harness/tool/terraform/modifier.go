package terraform

import (
	"fmt"

	"github.com/pluralsh/polly/algorithms"
)

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
	return append(args, fmt.Sprintf(in.planFileName))
}

func NewApplyModifier(planFileName string) *ApplyModifier {
	return &ApplyModifier{planFileName}
}
