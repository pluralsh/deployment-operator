package trivy

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
	loglevel "github.com/pluralsh/deployment-operator/pkg/log"
)

type Scanner struct {
	v1.DefaultScanner
}

func (in *Scanner) Scan(tool console.StackType, options ...v1.ScanOption) (json string, err error) {
	opts := &v1.ScanOptions{}
	for _, option := range options {
		option(opts)
	}

	return in.scan(tool, opts)
}

func (in *Scanner) scan(tool console.StackType, options *v1.ScanOptions) (json string, err error) {
	switch tool {
	case console.StackTypeTerraform:
		return in.scanTerraform(options)
	default:
		klog.Fatalf("unsupported tool type: %s", tool)
		return "", nil
	}
}

func (in *Scanner) scanTerraform(options *v1.ScanOptions) (json string, err error) {
	args := []string{
		"config",
	}

	if len(in.PolicyPaths) > 0 {
		for _, path := range in.PolicyPaths {
			args = append(args, []string{"--config-check", path}...)
		}
	}

	args = append(args, []string{
		"-f", "json",
		"--scanners" ,"secret,misconfig",
		"--tf-vars", options.Terraform.VariablesFileName,
		options.Terraform.PlanFileName,
	}...)

	output, err := exec.NewExecutable(
		"trivy",
		exec.WithArgs(args),
		exec.WithDir(options.Terraform.Dir),
	).RunWithOutput(context.Background())

	klog.V(loglevel.LogLevelVerbose).InfoS("trivy output", "output", string(output))
	return string(output), err
}

func New(policyPaths []string) v1.Scanner {
	return &Scanner{DefaultScanner: v1.DefaultScanner{PolicyPaths: policyPaths}}
}
