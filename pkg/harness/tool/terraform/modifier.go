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
	result := append(args, fmt.Sprintf("-out=%s", in.planFileName))
	if in.parallelism != nil {
		result = append(result, fmt.Sprintf("-parallelism=%d", *in.parallelism))
	}
	if in.refresh != nil {
		result = append(result, fmt.Sprintf("-refresh=%t", *in.refresh))
	}
	return result
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

	result := append(args, in.planFileName)
	if in.parallelism != nil {
		result = append(result, fmt.Sprintf("-parallelism=%d", *in.parallelism))
	}
	if in.refresh != nil {
		result = append(result, fmt.Sprintf("-refresh=%t", *in.refresh))
	}
	return result
}

func NewApplyArgsModifier(dir, planFileName string, parallelism *int64, refresh *bool) v1.Modifier {
	return &ApplyArgsModifier{planFileName: planFileName, dir: dir, parallelism: parallelism, refresh: refresh}
}
