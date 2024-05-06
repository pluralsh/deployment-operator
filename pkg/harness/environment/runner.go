package environment

import (
	console "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2"
)

func newRunner(stackType console.StackType, dir string) runner {
	var r runner

	switch stackType {
	case console.StackTypeTerraform:
		r = newTerraformRunner(dir)
	case console.StackTypeAnsible:
		r = newAnsibleRunner(dir)
	default:
		klog.Fatalf("unsupported stack type: %s", stackType)
	}

	return r
}
