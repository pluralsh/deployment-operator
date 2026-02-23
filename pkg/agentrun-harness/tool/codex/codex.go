package codex

import (
	"context"
	"fmt"
	"path"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/log"
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

	//promptFile := path.Join(in.Config.WorkDir, ".codex", "prompts", v1.SystemPromptFile)

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

	cfg, err := BuildCodexConfig(in.Config.WorkDir, agents, mcps)
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

	args = []string{"exec", "--profile", agent, "--skip-git-repo-check", "true", "--json"}

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
		klog.Info(string(line))
	})
	if err != nil {
		klog.ErrorS(err, "claude execution failed")
		in.Config.ErrorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("claude execution finished")

	close(in.Config.FinishedChan)
}
