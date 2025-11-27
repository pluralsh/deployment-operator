package claude

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type Model string

const (
	ClaudeSonnet45 Model = "claude-sonnet-4-5-20250929"
	ClaudeSonnet4  Model = "claude-sonnet-4-20250514"
	ClaudeOpus45   Model = "claude-opus-4-5-20251101"
	ClaudeOpus4    Model = "claude-opus-4-20250514"
	ClaudeOpus41   Model = "claude-opus-4-1-20250805"
)

func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvClaudeModel, string(ClaudeOpus45)) {
	case string(ClaudeOpus45):
		return ClaudeOpus45
	case string(ClaudeOpus4):
		return ClaudeOpus4
	case string(ClaudeOpus41):
		return ClaudeOpus41
	case string(ClaudeSonnet4):
		return ClaudeSonnet4
	case string(ClaudeSonnet45):
		return ClaudeSonnet45
	default:
		return ClaudeOpus45
	}
}

type Claude struct {
	// dir is a working directory used to run opencode.
	dir string

	// repositoryDir is a directory where the cloned repository is located.
	repositoryDir string

	// run is the agent run that is being processed.
	run *v1.AgentRun

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)

	// executable is the claude executable used to call CLI.
	executable exec.Executable

	// token is the token used to authenticate with the API.
	token string

	// model is the model used to generate code.
	model Model
}
