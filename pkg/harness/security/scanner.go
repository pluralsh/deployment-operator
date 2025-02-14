package security

import (
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/security/trivy"
	"github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
)

func NewScanner(t v1.ScannerType, policyPaths []string) v1.Scanner {
	var s v1.Scanner

	switch t {
	case v1.ScannerTypeTrivy:
		s = trivy.New(policyPaths)
	default:
		klog.Fatalf("unsupported scanner type: %s", t)
	}

	return s
}
