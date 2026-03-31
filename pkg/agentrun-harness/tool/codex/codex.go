package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

const consoleTokenEnv = "PLRL_CONSOLE_TOKEN"

func New(config v1.Config) v1.Tool {
	result := &Codex{
		DefaultTool: v1.DefaultTool{Config: config},
		apiKey:      config.Run.Runtime.Config.Codex.ApiKey,
		model:       EnsureModel(config.Run.Runtime.Config.Codex.Model),
		proxy:       config.Run.IsProxyEnabled(),
	}

	if config.Run.PluralCreds != nil {
		result.consoleToken = lo.FromPtr(config.Run.PluralCreds.Token)
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize codex tool: %v", err)
	}

	return result
}

func (in *Codex) ensure() error {
	if len(in.Config.WorkDir) == 0 {
		return fmt.Errorf("work directory is not set")
	}

	if len(in.Config.RepositoryDir) == 0 {
		return fmt.Errorf("repository directory is not set")
	}

	if !in.proxy && len(in.apiKey) == 0 {
		return fmt.Errorf("codex API key is not set")
	}
	return nil
}

func (in *Codex) Run(ctx context.Context, options ...exec.Option) {
	go in.start(ctx, options...)
}

func (in *Codex) Configure(consoleURL, consoleToken, deployToken string) error {
	if err := in.ConfigureSystemPrompt(console.AgentRuntimeTypeCodex); err != nil {
		return err
	}

	baseAgent := AgentInput{
		Model:                string(in.model),
		ApprovalPolicy:       "never",
		ModelReasoningEffort: "medium",
		AllowedEnvVars:       []string{"PATH", "HOME"},
		EnableWebSearch:      true,
		EnableShellCache:     true,
	}

	// The plural proxy requires models in "provider/model" format.
	if in.proxy {
		baseAgent.Model = "openai/" + baseAgent.Model
	}

	var (
		agents    []AgentInput
		mcps      []MCPInput
		providers []ModelProviderInput
	)

	modelProvider := ""
	if in.proxy {
		modelProvider = "plural"
		providers = []ModelProviderInput{{
			Name:    "plural",
			BaseURL: fmt.Sprintf("%s/ext/ai/v1", consoleURL),
			EnvKey:  consoleTokenEnv,
		}}
	}

	switch in.Config.Run.Mode {
	case console.AgentRunModeAnalyze:
		agents = []AgentInput{{
			Name:                 "analysis",
			SandboxMode:          "read-only",
			Model:                baseAgent.Model,
			ApprovalPolicy:       baseAgent.ApprovalPolicy,
			ModelReasoningEffort: baseAgent.ModelReasoningEffort,
			AllowedEnvVars:       baseAgent.AllowedEnvVars,
			ModelProvider:        modelProvider,
		}}
		mcps = []MCPInput{{
			Name:         "plural",
			Type:         "stdio",
			Command:      "/usr/local/bin/mcpserver",
			EnabledTools: []string{"updateAgentRunAnalysis"},
			Env: map[string]string{
				consoleTokenEnv:     consoleToken,
				"PLRL_CONSOLE_URL":  consoleURL,
				"PLRL_AGENT_RUN_ID": in.Config.Run.ID,
			},
		}}
	case console.AgentRunModeWrite:
		agents = []AgentInput{{
			Name:                 "autonomous",
			SandboxMode:          "workspace-write",
			Model:                baseAgent.Model,
			ApprovalPolicy:       baseAgent.ApprovalPolicy,
			ModelReasoningEffort: baseAgent.ModelReasoningEffort,
			AllowedEnvVars:       baseAgent.AllowedEnvVars,
			ModelProvider:        modelProvider,
		}}
		mcps = []MCPInput{{
			Name:         "plural",
			Type:         "stdio",
			Command:      "/usr/local/bin/mcpserver",
			EnabledTools: []string{"agentPullRequest", "createBranch", "fetchAgentRunTodos", "updateAgentRunTodos"},
			Env: map[string]string{
				consoleTokenEnv:     consoleToken,
				"PLRL_CONSOLE_URL":  consoleURL,
				"PLRL_AGENT_RUN_ID": in.Config.Run.ID,
			},
		}}
	default:
		return fmt.Errorf("unsupported agent run mode %q for codex", in.Config.Run.Mode)
	}

	cfg, err := BuildCodexConfig(in.Config.WorkDir, agents, mcps, providers)
	if err != nil {
		return err
	}

	config, err := WriteCodexConfig(path.Join(in.Config.WorkDir, ".codex"), cfg)
	if err != nil {
		return err
	}

	klog.InfoS("Codex configured", "configPath", config)

	return nil
}

func (in *Codex) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func (in *Codex) start(ctx context.Context, options ...exec.Option) {
	// In proxy mode the plural provider handles auth via PLRL_CONSOLE_TOKEN;
	// codex login is only needed for direct OpenAI usage.
	if !in.proxy {
		loginArgs := []string{"-c", "printenv OPENAI_API_KEY | codex login --with-api-key"}
		in.executable = exec.NewExecutable(
			"bash",
			append(
				options,
				exec.WithArgs(loginArgs),
				exec.WithDir(in.Config.WorkDir),
				exec.WithEnv([]string{fmt.Sprintf("OPENAI_API_KEY=%s", in.apiKey), fmt.Sprintf("CODEX_HOME=%s", path.Join(in.Config.WorkDir, ".codex"))}),
				exec.WithTimeout(in.Config.Run.Runtime.Config.Codex.Timeout),
			)...,
		)
		if err := in.executable.Run(ctx); err != nil {
			klog.ErrorS(err, "codex login failed")
			in.Config.ErrorChan <- err
			return
		}
	}

	agent := "analysis"
	if in.Config.Run.Mode == console.AgentRunModeWrite {
		agent = "autonomous"
	}

	args := []string{"exec", "--profile", agent, "--skip-git-repo-check", "--json", in.Config.Run.Prompt}

	in.executable = exec.NewExecutable(
		"codex",
		append(
			options,
			exec.WithArgs(args),
			exec.WithDir(in.Config.WorkDir),
			exec.WithEnv([]string{fmt.Sprintf("PLRL_CONSOLE_TOKEN=%s", in.consoleToken), fmt.Sprintf("CODEX_HOME=%s", path.Join(in.Config.WorkDir, ".codex"))}),
			exec.WithTimeout(in.Config.Run.Runtime.Config.Codex.Timeout),
		)...,
	)

	// Send the initial prompt as a message too
	if in.onMessage != nil {
		in.onMessage(&console.AgentMessageAttributes{Message: in.Config.Run.Prompt, Role: console.AiRoleUser})
	}

	err := in.executable.RunStream(ctx, func(line []byte) {
		event := &StreamEvent{}
		if err := json.Unmarshal(line, event); err != nil {
			klog.V(log.LogLevelExtended).InfoS("failed to unmarshal codex stream event", "line", string(line))
			return
		}

		if event.Type == "thread.started" && event.ThreadID != "" {
			in.threadID = event.ThreadID
			klog.V(log.LogLevelDebug).InfoS("codex thread started", "thread_id", in.threadID)
		}

		msg := mapCodexStreamEventToAgentMessage(event, in.threadID)
		if in.onMessage != nil && msg != nil {
			in.onMessage(msg)
		}
	})
	if err != nil {
		klog.ErrorS(err, "codex execution failed")
		in.Config.ErrorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("codex execution finished")

	close(in.Config.FinishedChan)
}

// mapCodexStreamEventToAgentMessage converts a single Codex CLI JSON stream event into an
// AgentMessageAttributes to be forwarded to the API.
func mapCodexStreamEventToAgentMessage(event *StreamEvent, threadID string) *console.AgentMessageAttributes {
	switch event.Type {
	case "item.completed":
		if event.Item == nil {
			return nil
		}
		return mapStreamItem(event.Item, threadID)
	case "turn.completed":
		if event.Usage == nil {
			return nil
		}
		totalTokens := float64(event.Usage.InputTokens + event.Usage.OutputTokens)
		return &console.AgentMessageAttributes{
			Role:    console.AiRoleAssistant,
			Message: "turn.completed",
			Cost: &console.AgentMessageCostAttributes{
				Total: totalTokens,
				Tokens: &console.AgentMessageTokensAttributes{
					Input:  lo.ToPtr(float64(event.Usage.InputTokens)),
					Output: lo.ToPtr(float64(event.Usage.OutputTokens)),
				},
			},
		}
	}
	return nil
}

// mapStreamItem maps a completed StreamItem to an AgentMessageAttributes.
func mapStreamItem(item *StreamItem, threadID string) *console.AgentMessageAttributes {
	switch item.Type {
	case "reasoning":
		if item.Text == "" {
			return nil
		}
		klog.V(log.LogLevelDebug).InfoS("codex reasoning", "text", item.Text, "thread_id", threadID)
		return &console.AgentMessageAttributes{
			Role:    console.AiRoleAssistant,
			Message: item.Text,
		}

	case "command_execution":
		if item.Status != "completed" && item.Status != "failed" {
			return nil
		}
		exitCode := 0
		if item.ExitCode != nil {
			exitCode = *item.ExitCode
		}
		state := console.AgentMessageToolStateCompleted
		if item.Status == "failed" || exitCode != 0 {
			state = console.AgentMessageToolStateError
		}
		klog.V(log.LogLevelDebug).InfoS("codex command execution", "command", item.Command, "exit_code", exitCode, "thread_id", threadID)
		return &console.AgentMessageAttributes{
			Role:    console.AiRoleAssistant,
			Message: "Called tool",
			Metadata: &console.AgentMessageMetadataAttributes{
				Tool: &console.AgentMessageToolAttributes{
					Name:   lo.ToPtr(item.Command),
					State:  lo.ToPtr(state),
					Output: lo.ToPtr(item.AggregatedOutput),
				},
			},
		}
	}

	return nil
}
