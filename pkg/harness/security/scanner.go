package security

import (
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/security/trivy"
	"github.com/pluralsh/deployment-operator/pkg/harness/security/v1"
)



//func WithPolicy(policies ...string) v1.Option {
//	return func(s v1.Scanner) {
//		switch t := s.(type) {
//		case *trivy.Scanner:
//			for _, o := range options {
//				o(t)
//			}
//		default:
//			klog.Fatalf("unknown scanner type: %T", t)
//		}
//	}
//}



func NewScanner(t v1.Type, options ...v1.Option) v1.Scanner {
	var s v1.Scanner

	switch t {
	case v1.TypeTrivy:
		s = trivy.New()
	default:
		klog.Fatalf("unsupported scanner type: %s", t)
	}

	for _, option := range options {
		option(s)
	}

	return s
}
