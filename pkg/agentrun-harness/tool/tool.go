package tool

import (
	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/opencode"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
)

// New creates a specific tool implementation structure based on the provided
// console.AgentRuntimeType
func New(stackType console.AgentRuntimeType, config v1.Config) v1.Tool {
	// TODO: detect if runtime has proxy enabled as right now only that is supported
	var t v1.Tool
	switch stackType {
	case console.AgentRuntimeTypeOpencode:
		t = opencode.New(config)
	default:
		klog.Fatalf("unsupported stack type: %s", stackType)
	}

	return t
}
