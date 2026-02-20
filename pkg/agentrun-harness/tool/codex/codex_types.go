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
	token string

	// model used to generate code.
	model Model
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
	Profiles   map[string]*Profile   `toml:"profiles"`
	MCPServers map[string]*MCPServer `toml:"mcp_servers"`
}
