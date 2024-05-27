package terraform

import (
	"fmt"
	"path"

	"github.com/samber/lo"

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
	if !lo.Contains(args, "plan") {
		return args
	}

	return append(args, fmt.Sprintf("-out=%s", in.planFileName))
}

func NewPlanModifier(planFileName string) *PlanModifier {
	return &PlanModifier{planFileName}
}

func (in *ApplyModifier) Args(args []string) []string {
	if !lo.Contains(args, "apply") {
		return args
	}

	if !helpers.Exists(path.Join(in.dir, in.planFileName)) || lo.Contains(args, in.planFileName) {
		return args
	}

	return append(args, in.planFileName)
}

func NewApplyModifier(dir, planFileName string) *ApplyModifier {
	return &ApplyModifier{planFileName, dir}
}
