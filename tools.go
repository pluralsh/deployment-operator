//go:build tools

package tools

import (
	_ "github.com/elastic/crd-ref-docs"
	_ "github.com/vektra/mockery/v2"
	_ "sigs.k8s.io/controller-runtime/tools/setup-envtest"
)
