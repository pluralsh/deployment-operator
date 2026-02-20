package codex

import (
	console "github.com/pluralsh/console/go/client"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

type Codex struct {
	v1.DefaultTool

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)

	// executable is the Gemini executable used to call CLI.
	executable exec.Executable

	// apiKey used to authenticate with the API.
	apiKey string

	// model used to generate code.
	model Model
}
