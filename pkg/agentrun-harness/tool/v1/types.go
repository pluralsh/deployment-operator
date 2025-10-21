package v1

import (
	"context"

	console "github.com/pluralsh/console/go/client"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

// Tool handles one of the supported AI agents CLI tools.
// The list of supported tools is based on the console.AgentRuntimeType.
type Tool interface {
	// Run executes the tool in the background. Use Config to get the tool configuration
	// and signal when the tool is finished or failed.
	Run(ctx context.Context, options ...exec.Option)

	// Configure configures the tool.
	Configure(consoleURL, deployToken, consoleToken string) error

	// Messages returns messages produced by the tool during its execution.
	// Make sure that the tool is finished before calling this method, or it will
	// return an empty list.
	Messages() []*console.AgentMessageAttributes
}

type Config struct {
	// WorkDir is the working directory for the tool.
	WorkDir string

	// RepositoryDir is the directory where the cloned repository is located.
	RepositoryDir string

	// FinishedChan is a channel that gets closed when the tool is finished.
	FinishedChan chan struct{}

	// ErrorChan is a channel that returns an error if the tool failed
	// and immediately closes.
	ErrorChan chan error

	// Run is the agent run that is being processed.
	Run *v1.AgentRun
}
