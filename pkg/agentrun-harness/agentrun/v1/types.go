package v1

import (
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

const (
	// defaultTimeout is the default timeout for AI provider CLI execution.
	defaultTimeout = 120 * time.Minute

	// defaultBashTimeout is the default timeout for any bash command Claude executes.
	defaultBashTimeout = 30 * time.Minute

	// defaultBashMaxTimeout is the maximum time Claude is permitted to wait
	// for a command before it is terminated.
	defaultBashMaxTimeout = defaultTimeout

	// defaultInactivityTimeout is the default Gemini CLI timeout for the process,
	// tool call, or session if there is no output or input detected.
	defaultInactivityTimeout = defaultBashTimeout
)

type AgentRun struct {
	ID         string                 `json:"id"`
	Prompt     string                 `json:"prompt"`
	Repository string                 `json:"repository"`
	Mode       console.AgentRunMode   `json:"mode"`
	Status     console.AgentRunStatus `json:"status"`
	FlowID     *string                `json:"flowId,omitempty"`

	// Credentials for SCM and Plural Console
	ScmCreds    *console.ScmCredentialFragment `json:"scmCreds,omitempty"`
	PluralCreds *console.PluralCredsFragment   `json:"pluralCreds,omitempty"`

	// Runtime information
	Runtime *AgentRuntime `json:"runtime,omitempty"`

	DindEnabled    bool
	BrowserEnabled bool
}

type AgentRuntime struct {
	ID      string                   `json:"id"`
	Name    string                   `json:"name"`
	Type    console.AgentRuntimeType `json:"type"`
	AiProxy bool                     `json:"aiProxy"`
	Config  *AgentRuntimeConfig      `json:"config,omitempty"`
}

type AgentRuntimeConfig struct {
	Claude   *ClaudeConfig   `json:"claude,omitempty"`
	OpenCode *OpencodeConfig `json:"opencode,omitempty"`
	Gemini   *GeminiConfig   `json:"gemini,omitempty"`
	Codex    *CodexConfig    `json:"codex,omitempty"`
}

type OpencodeConfig struct {
	Provider string        `json:"provider"`
	Endpoint string        `json:"endpoint"`
	Model    string        `json:"model,omitempty"`
	Token    string        `json:"tokenSecretRef"`
	Timeout  time.Duration `json:"timeout,omitempty"`
}

type ClaudeConfig struct {
	ApiKey         string        `json:"apiKey"`
	Model          string        `json:"model,omitempty"`
	ExtraArgs      []string      `json:"extraArgs,omitempty"`
	Timeout        time.Duration `json:"timeout"`
	BashTimeout    time.Duration `json:"bashTimeout"`
	BashMaxTimeout time.Duration `json:"bashMaxTimeout"`
}

type GeminiConfig struct {
	APIKey            string        `json:"apiKey"`
	Model             string        `json:"model,omitempty"`
	Timeout           time.Duration `json:"timeout"`
	InactivityTimeout time.Duration `json:"inactivityTimeout"`
}

type CodexConfig struct {
	ApiKey  string        `json:"apiKey"`
	Model   string        `json:"model,omitempty"`
	Timeout time.Duration `json:"timeout"`
}

// FromAgentRunFragment converts Console API fragment to harness type
func (ar *AgentRun) FromAgentRunFragment(fragment *console.AgentRunFragment) *AgentRun {
	run := &AgentRun{
		ID:          fragment.ID,
		Prompt:      fragment.Prompt,
		Repository:  fragment.Repository,
		Mode:        fragment.Mode,
		Status:      fragment.Status,
		ScmCreds:    fragment.ScmCreds,
		PluralCreds: fragment.PluralCreds,
		Runtime:     &AgentRuntime{},
	}

	if fragment.Flow != nil {
		run.FlowID = &fragment.Flow.ID
	}

	run.Runtime = ar.fromEnv(fragment.Runtime)

	if helpers.GetPluralEnvBool(controller.EnvDindEnabled, false) {
		run.DindEnabled = true
	}

	if helpers.GetPluralEnvBool(controller.EnvBrowserEnabled, false) {
		run.BrowserEnabled = true
	}

	return run
}

func (ar *AgentRun) fromEnv(runtime *console.AgentRuntimeFragment) *AgentRuntime {
	result := &AgentRuntime{}

	if runtime == nil {
		return result
	}

	result.ID = runtime.ID
	result.Name = runtime.Name
	result.Type = runtime.Type
	result.AiProxy = runtime.AiProxy != nil && *runtime.AiProxy

	config := &AgentRuntimeConfig{}
	switch runtime.Type {
	case console.AgentRuntimeTypeClaude:
		config.Claude = &ClaudeConfig{
			ApiKey:         helpers.GetEnv(controller.EnvClaudeToken, ""),
			Model:          helpers.GetPluralEnv(controller.EnvClaudeModel, ""),
			ExtraArgs:      helpers.GetPluralEnvSlice(controller.EnvClaudeArgs, nil),
			Timeout:        helpers.GetPluralEnvDuration(controller.EnvExecTimeout, defaultTimeout),
			BashTimeout:    helpers.GetPluralEnvDuration(controller.EnvClaudeBashDefaultTimeout, defaultBashTimeout),
			BashMaxTimeout: helpers.GetPluralEnvDuration(controller.EnvClaudeBashMaxTimeout, defaultBashMaxTimeout),
		}
	case console.AgentRuntimeTypeOpencode:
		config.OpenCode = &OpencodeConfig{
			Provider: helpers.GetPluralEnv(controller.EnvOpenCodeProvider, ""),
			Endpoint: helpers.GetPluralEnv(controller.EnvOpenCodeEndpoint, ""),
			Model:    helpers.GetPluralEnv(controller.EnvOpenCodeModel, ""),
			Token:    helpers.GetPluralEnv(controller.EnvOpenCodeToken, ""),
			Timeout:  helpers.GetPluralEnvDuration(controller.EnvExecTimeout, defaultTimeout),
		}
	case console.AgentRuntimeTypeGemini:
		config.Gemini = &GeminiConfig{
			APIKey:            helpers.GetPluralEnv(controller.EnvGeminiAPIKey, ""),
			Model:             helpers.GetPluralEnv(controller.EnvGeminiModel, ""),
			Timeout:           helpers.GetPluralEnvDuration(controller.EnvExecTimeout, defaultTimeout),
			InactivityTimeout: helpers.GetPluralEnvDuration(controller.EnvGeminiInactivityTimeout, defaultInactivityTimeout),
		}
	case console.AgentRuntimeTypeCodex:
		config.Codex = &CodexConfig{
			ApiKey:  helpers.GetPluralEnv(controller.EnvCodexAPIKey, ""),
			Model:   helpers.GetPluralEnv(controller.EnvCodexModel, ""),
			Timeout: helpers.GetPluralEnvDuration(controller.EnvExecTimeout, defaultTimeout),
		}
	}

	result.Config = config
	return result
}

func (ar *AgentRun) IsProxyEnabled() bool {
	return ar.Runtime != nil && ar.Runtime.AiProxy
}
