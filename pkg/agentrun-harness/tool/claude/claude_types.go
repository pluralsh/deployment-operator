package claude

import (
	console "github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	toolv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
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
	toolv1.DefaultTool

	// dir is a working directory used to run opencode.
	//dir string

	// repositoryDir is a directory where the cloned repository is located.
	//repositoryDir string

	// run is the agent run that is being processed.
	//run *v1.AgentRun

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)

	// executable is the claude executable used to call CLI.
	executable exec.Executable

	// token is the token used to authenticate with the API.
	token string

	// model is the model used to generate code.
	model Model

	// errorChan is a channel that returns an error if the tool failed
	//errorChan chan error

	// finishedChan is a channel that gets closed when the tool is finished.
	//finishedChan chan struct{}

	// startedChan is a channel that gets closed when the opencode server is started.
	//startedChan chan struct{}

	// toolUseCache maps tool_use id to ContentMsg for resolving tool_result.
	toolUseCache map[string]ContentMsg
}

type StreamEvent struct {
	Type    string        `json:"type"`
	Message *MessageEvent `json:"message,omitempty"`
	// there are other event types but you only need `message` for now
	SessionID       string `json:"session_id"`
	UUID            string `json:"uuid"`
	ParentToolUseID string `json:"parent_tool_use_id"`
}

type MessageEvent struct {
	Model        string       `json:"model"`
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Role         string       `json:"role"`
	StopReason   *string      `json:"stop_reason"`
	StopSequence *string      `json:"stop_sequence"`
	Usage        *Usage       `json:"usage"`
	Content      []ContentMsg `json:"content"`
}

type ContentMsg struct {
	Type string `json:"type"` // "text", "tool_use", "tool_result"
	Text string `json:"text,omitempty"`

	// Fields for tool_use
	ID    string                 `json:"id,omitempty"`    // Unique tool invocation ID
	Name  string                 `json:"name,omitempty"`  // Tool name (e.g., "web_search")
	Input map[string]interface{} `json:"input,omitempty"` // Tool input parameters

	// Fields for tool_result
	ToolUseID string      `json:"tool_use_id,omitempty"` // References the tool_use ID
	Content   interface{} `json:"content,omitempty"`     // Tool output (can be string or structured)
	IsError   bool        `json:"is_error,omitempty"`    // Whether tool execution failed
}

type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}
