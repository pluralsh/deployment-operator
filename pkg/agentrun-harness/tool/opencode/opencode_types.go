package opencode

import (
	"context"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
)

const (
	defaultOpenCodePort  = "4096"
	defaultModelID       = "gpt-4.1-mini"
	defaultModelName     = "GPT 4.1 Mini"
	defaultProviderID    = "openai-proxy"
	defaultProviderName  = "OpenAI Proxy"
	defaultAnalysisAgent = "plural-reviewer"
	defaultWriteAgent    = "plural-writer"
)

// Opencode implements v1.Tool interface.
type Opencode struct {
	// dir is a working directory used to run opencode.
	dir string

	// repositoryDir is a directory where the cloned repository is located.
	repositoryDir string

	// port is a port the opencode server will listen on.
	port string

	// run is the agent run that is being processed.
	run *v1.AgentRun

	// contextCancel is a function that cancels the internal context
	contextCancel context.CancelCauseFunc

	// errorChan is a channel that returns an error if the tool failed
	errorChan chan error

	// finishedChan is a channel that gets closed when the tool is finished.
	finishedChan chan struct{}

	// startedChan is a channel that gets closed when the opencode server is started.
	startedChan chan struct{}
}
