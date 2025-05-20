package terraform

import (
	"fmt"
	"path"

	"github.com/samber/lo"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

// Args implements [v1.ArgsModifier] type.
func (in *PlanArgsModifier) Args(args []string) []string {
	if !lo.Contains(args, "plan") {
		return args
	}
	args = append(args, fmt.Sprintf("-out=%s", in.planFileName))
	if in.parallelism != nil {
		args = append(args, fmt.Sprintf("-parallelism=%d", *in.parallelism))
	}
	if in.refresh != nil {
		args = append(args, fmt.Sprintf("-refresh=%t", *in.refresh))
	}
	return args
}

func NewPlanArgsModifier(planFileName string, parallelism *int64, refresh *bool) v1.Modifier {
	return &PlanArgsModifier{planFileName: planFileName, parallelism: parallelism, refresh: refresh}
}

// Args implements [v1.ArgsModifier] type.
func (in *ApplyArgsModifier) Args(args []string) []string {
	if !lo.Contains(args, "apply") {
		return args
	}

	if !helpers.Exists(path.Join(in.dir, in.planFileName)) || lo.Contains(args, in.planFileName) {
		return args
	}

	args = append(args, in.planFileName)
	if in.parallelism != nil {
		args = append(args, fmt.Sprintf("-parallelism=%d", *in.parallelism))
	}
	if in.refresh != nil {
		args = append(args, fmt.Sprintf("-refresh=%t", *in.refresh))
	}
	return args
}

func NewApplyArgsModifier(dir, planFileName string, parallelism *int64, refresh *bool) v1.Modifier {
	return &ApplyArgsModifier{planFileName: planFileName, dir: dir, parallelism: parallelism, refresh: refresh}
}
