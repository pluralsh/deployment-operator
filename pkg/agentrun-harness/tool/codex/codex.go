package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

func New(config v1.Config) v1.Tool {
	result := &Codex{
		DefaultTool: v1.DefaultTool{Config: config},
		apiKey:      helpers.GetEnv(controller.EnvCodexAPIKey, ""),
		model:       DefaultModel(),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize claude tool: %v", err)
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

	if len(in.apiKey) == 0 {
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

	agents := []AgentInput{
		{
			Name:                 "analysis",
			Model:                string(in.model),
			SandboxMode:          "read-only",
			ApprovalPolicy:       "never",
			ModelReasoningEffort: "medium",
			AllowedEnvVars:       []string{"PATH", "HOME"},
			EnableWebSearch:      true,
			EnableShellCache:     true,
			//PromptFile:           promptFile,
			EnabledTools: []string{
				"Read", "Grep", "Glob",
				"Bash(ls:*)", "Bash(cd:*)", "Bash(pwd)",
				"WebFetch", "updateAgentRunAnalysis",
			},
			DisabledTools: []string{
				"Edit", "Write", "Bash(rm:*)", "Bash(sudo:*)",
			},
		},
		{
			Name:                 "autonomous",
			Model:                string(in.model),
			SandboxMode:          "workspace-write",
			ApprovalPolicy:       "never",
			ModelReasoningEffort: "medium",
			AllowedEnvVars:       []string{"PATH", "HOME"},
			EnableWebSearch:      true,
			EnableShellCache:     true,
			//PromptFile:           promptFile,
			EnabledTools: []string{
				"Read", "Write", "Edit", "MultiEdit", "Bash", "WebFetch",
				"agentPullRequest",
				"createBranch",
				"fetchAgentRunTodos",
				"updateAgentRunTodos",
			},
		},
	}

	mcps := []MCPInput{
		{
			Name:    "plural",
			Command: "mcpserver",
			Env: map[string]string{
				"PLRL_CONSOLE_TOKEN": consoleToken,
				"PLRL_CONSOLE_URL":   consoleURL,
			},
		},
	}

	cfg, err := BuildCodexConfig(path.Join(in.Config.WorkDir, "shared"), agents, mcps)
	if err != nil {
		return err
	}

	config, err := WriteCodexConfig(path.Join(in.Config.WorkDir, ".codex"), cfg)
	if err != nil {
		return err
	}

	klog.Info("Codex configured", "configPath", config)

	return nil
}

func (in *Codex) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func (in *Codex) start(ctx context.Context, options ...exec.Option) {
	// CODEX_HOME must be set to the directory where the codex config is located for the agent CLI to pick it up during the run.
	args := []string{"-c", "printenv OPENAI_API_KEY | codex login --with-api-key"}

	in.executable = exec.NewExecutable(
		"bash",
		append(
			options,
			exec.WithArgs(args),
			exec.WithDir(in.Config.WorkDir),
			exec.WithEnv([]string{fmt.Sprintf("OPENAI_API_KEY=%s", in.apiKey), fmt.Sprintf("CODEX_HOME=%s", path.Join(in.Config.WorkDir, ".codex"))}),
			exec.WithTimeout(15*time.Minute),
		)...,
	)
	if err := in.executable.Run(ctx); err != nil {
		klog.ErrorS(err, "codex login failed")
		in.Config.ErrorChan <- err
		return
	}

	agent := "analysis"
	if in.Config.Run.Mode == console.AgentRunModeWrite {
		agent = "autonomous"
	}

	args = []string{"exec", "--profile", agent, "--skip-git-repo-check", "true", "--cd", path.Join(in.Config.WorkDir, "shared"), "--json"}

	in.executable = exec.NewExecutable(
		"codex",
		append(
			options,
			exec.WithArgs(args),
			exec.WithDir(in.Config.WorkDir),
			exec.WithEnv([]string{fmt.Sprintf("CODEX_HOME=%s", path.Join(in.Config.WorkDir, ".codex"))}),
			exec.WithTimeout(15*time.Minute),
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

		// Capture thread_id from the "thread.started" event so it can be
		// forwarded to the API (analogous to session_id in Claude).
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
		klog.ErrorS(err, "claude execution failed")
		in.Config.ErrorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("claude execution finished")

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
					Name:   lo.ToPtr("shell"),
					State:  lo.ToPtr(state),
					Input:  lo.ToPtr(item.Command),
					Output: lo.ToPtr(item.AggregatedOutput),
				},
			},
		}
	}

	return nil
}
