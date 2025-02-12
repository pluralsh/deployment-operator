package tool

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	securityv1 "github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/tool/ansible"
	"github.com/pluralsh/deployment-operator/pkg/harness/tool/terraform"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

type Config struct {
	WorkDir   string
	ExecDir   string
	Variables *string
	Scanner   securityv1.Scanner
}

func ensure(stackType console.StackType, config Config) {
	// No args are required for custom runner
	if stackType == console.StackTypeCustom {
		return
	}

	if len(config.ExecDir) == 0 {
		klog.Fatal("execdir is empty")
	}

	// Above args are required for non-custom runners
	if stackType == console.StackTypeTerraform {
		return
	}

	if len(config.WorkDir) == 0 {
		klog.Fatal("workdir is empty")
	}
}

// New creates a specific tool implementation structure based on the provided
// gqlclient.StackType.
func New(stackType console.StackType, config Config) v1.Tool {
	ensure(stackType, config)

	var t v1.Tool
	switch stackType {
	case console.StackTypeTerraform:
		t = terraform.New(config.ExecDir, config.Variables)
	case console.StackTypeAnsible:
		t = ansible.New(config.WorkDir, config.ExecDir)
	case console.StackTypeCustom:
		t = v1.New()
	default:
		klog.Fatalf("unsupported stack type: %s", stackType)
	}

	return t
}
