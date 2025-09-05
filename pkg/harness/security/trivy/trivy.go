package trivy

import (
	"context"
	"encoding/json"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
	loglevel "github.com/pluralsh/deployment-operator/pkg/log"
)

// Scan implements [v1.Scanner.Scan] interface.
func (in *Scanner) Scan(tool console.StackType, options ...v1.ScanOption) ([]*console.StackPolicyViolationAttributes, error) {
	opts := &v1.ScanOptions{}
	for _, option := range options {
		option(opts)
	}

	return in.scan(tool, opts)
}

// scan performs the actual scan for a given tool.
func (in *Scanner) scan(tool console.StackType, options *v1.ScanOptions) ([]*console.StackPolicyViolationAttributes, error) {
	switch tool {
	case console.StackTypeTerraform:
		return in.scanTerraform(options)
	default:
		klog.Fatalf("unsupported tool type: %s", tool)
		return nil, nil
	}
}

// scanTerraform performs a scan for Terraform.
func (in *Scanner) scanTerraform(options *v1.ScanOptions) ([]*console.StackPolicyViolationAttributes, error) {
	args := []string{
		"config",
	}

	if len(in.PolicyPaths) > 0 {
		args = append(args, []string{
			"--config-check", strings.Join(in.PolicyPaths, ","),
			"--check-namespaces", strings.Join(in.PolicyNamespaces, ","),
		}...)
	}

	args = append(args, []string{
		"-f", "json",
		"-q",
		"--tf-vars", options.Terraform.VariablesFileName,
		options.Terraform.PlanFileName,
	}...)

	output, err := exec.NewExecutable(
		"trivy",
		exec.WithArgs(args),
		exec.WithDir(options.Terraform.Dir),
	).RunWithOutput(context.Background())
	if err != nil {
		return nil, err
	}

	klog.V(loglevel.LogLevelTrace).InfoS("trivy output", "output", string(output))
	return in.toAttributes(output)
}

// toAttributes converts the Trivy output to a list of attributes.
func (in *Scanner) toAttributes(data []byte) ([]*console.StackPolicyViolationAttributes, error) {
	report := &Report{}
	if err := json.Unmarshal(data, report); err != nil {
		return nil, err
	}

	return report.Attributes(), nil
}

// New creates a new Trivy scanner.
func New(_ *console.PolicyEngineFragment) v1.Scanner {
	return &Scanner{DefaultScanner: v1.DefaultScanner{}}
}
