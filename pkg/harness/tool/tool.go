package tool

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/tool/ansible"
	"github.com/pluralsh/deployment-operator/pkg/harness/tool/terraform"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

// New creates a specific tool implementation structure based on the provided
// gqlclient.StackType.
func New(stackType console.StackType, workDir, execDir string) v1.Tool {
	var t v1.Tool

	switch stackType {
	case console.StackTypeTerraform:
		t = terraform.New(execDir)
	case console.StackTypeAnsible:
		t = ansible.New(workDir, execDir)
	case console.StackTypeCustom:
		t = v1.New()
	default:
		klog.Fatalf("unsupported stack type: %s", stackType)
	}

	return t
}
