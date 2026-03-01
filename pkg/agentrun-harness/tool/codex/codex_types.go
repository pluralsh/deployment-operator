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

	// executable is the Codex executable used to call CLI.
	executable exec.Executable

	// apiKey used to authenticate with the API.
	apiKey string

	// model used to generate code.
	model Model

	// threadID is captured from the "thread.started" event and forwarded to the API
	// as the session identifier (analogous to session_id in Claude).
	threadID string
}

// StreamEvent is the top-level envelope for every JSON line emitted by `codex exec --json`.
type StreamEvent struct {
	// Type identifies the event kind, e.g. "thread.started", "turn.started",
	// "item.started", "item.completed".
	Type string `json:"type"`

	// ThreadID is set on "thread.started" events and carries the session
	// identifier that must be forwarded to the API (analogous to session_id in Claude).
	ThreadID string `json:"thread_id,omitempty"`

	// Item is populated on "item.started" and "item.completed" events.
	Item *StreamItem `json:"item,omitempty"`
}

// StreamItem is the payload carried inside "item.started" / "item.completed" events.
type StreamItem struct {
	// ID is the stable identifier for this item across started/completed pairs.
	ID string `json:"id"`

	// Type describes what kind of item this is: "reasoning", "todo_list", "command_execution", etc.
	Type string `json:"type"`

	// Text is populated for "reasoning" items.
	Text string `json:"text,omitempty"`

	// Command and output fields are populated for "command_execution" items.
	Command          string `json:"command,omitempty"`
	AggregatedOutput string `json:"aggregated_output,omitempty"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	Status           string `json:"status,omitempty"`

	// Items is populated for "todo_list" items.
	Items []TodoItem `json:"items,omitempty"`
}

// TodoItem is a single entry inside a "todo_list" StreamItem.
type TodoItem struct {
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type AgentInput struct {
	Name                 string
	Model                string
	SandboxMode          string
	ApprovalPolicy       string
	ModelReasoningEffort string
	AllowedEnvVars       []string
	EnableWebSearch      bool
	EnableShellCache     bool
	PromptFile           string
	EnabledTools         []string
	DisabledTools        []string
}

type Project struct {
	TrustLevel string `toml:"trust_level,omitempty"`
}

type MCPInput struct {
	Name    string
	URL     string
	Command string
	Args    []string
	Env     map[string]string
}

type ShellEnvPolicy struct {
	IncludeOnly []string `toml:"include_only,omitempty"`
}

type Features struct {
	WebSearchRequest bool `toml:"web_search_request,omitempty"`
	ShellSnapshot    bool `toml:"shell_snapshot,omitempty"`
}

type Profile struct {
	Model                  string          `toml:"model"`
	SandboxMode            string          `toml:"sandbox_mode"`
	ApprovalPolicy         string          `toml:"approval_policy"`
	ModelReasoningEffort   string          `toml:"model_reasoning_effort"`
	ShellEnvironmentPolicy *ShellEnvPolicy `toml:"shell_environment_policy,omitempty"`
	Features               *Features       `toml:"features,omitempty"`
	Prompt                 string          `toml:"prompt,omitempty"`
	EnabledTools           []string        `toml:"enabled_tools,omitempty"`  // allow-list
	DisabledTools          []string        `toml:"disabled_tools,omitempty"` // deny-list
}

type MCPServer struct {
	URL     string            `toml:"url,omitempty"`     // For remote MCP
	Command string            `toml:"command,omitempty"` // For local MCP
	Args    []string          `toml:"args,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

type CodexConfig struct {
	Projects   map[string]*Project   `toml:"projects,omitempty"`
	Profiles   map[string]*Profile   `toml:"profiles"`
	MCPServers map[string]*MCPServer `toml:"mcp_servers"`
}
