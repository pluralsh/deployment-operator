package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
)

func New(config v1.Config) v1.Tool {
	result := &Claude{
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		run:           config.Run,
		token:         helpers.GetEnv(controller.EnvClaudeToken, ""),
		model:         DefaultModel(),
		finishedChan:  config.FinishedChan,
		errorChan:     config.ErrorChan,
		startedChan:   make(chan struct{}),
		toolUseCache:  make(map[string]ContentMsg),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize claude tool: %v", err)
	}

	return result
}

func (in *Claude) Run(ctx context.Context, options ...exec.Option) {
	go in.start(ctx, options...)
}

func (in *Claude) start(ctx context.Context, options ...exec.Option) {
	promptFile := path.Join(in.dir, ".claude", "prompts", "analysis.md")
	agent := analysisAgent
	if in.run.Mode == console.AgentRunModeWrite {
		agent = autonomousAgent
		promptFile = path.Join(in.dir, ".claude", "prompts", "autonomous.md")
	}
	args := []string{"--add-dir", in.repositoryDir, "--agents", agent, "--system-prompt-file", promptFile, "--model", string(DefaultModel()), "-p", in.run.Prompt, "--output-format", "stream-json", "--verbose"}

	in.executable = exec.NewExecutable(
		"claude",
		append(
			options,
			exec.WithArgs(args),
			exec.WithDir(in.dir),
			exec.WithEnv([]string{fmt.Sprintf("ANTHROPIC_API_KEY=%s", in.token)}),
			exec.WithTimeout(15*time.Minute),
		)...,
	)

	// Send the initial prompt as a message too
	if in.onMessage != nil {
		in.onMessage(&console.AgentMessageAttributes{Message: in.run.Prompt, Role: console.AiRoleUser})
	}

	err := in.executable.RunStream(ctx, func(line []byte) {
		event := &StreamEvent{}
		if err := json.Unmarshal(line, event); err != nil {
			klog.ErrorS(err, "failed to unmarshal claude stream event", "line", string(line))
			in.errorChan <- err
			return
		}

		if event.Message != nil {
			msg := mapClaudeContentToAgentMessage(event, in.toolUseCache)
			if in.onMessage != nil && msg != nil {
				in.onMessage(msg)
			}
		}
	})
	if err != nil {
		klog.ErrorS(err, "claude execution failed")
		in.errorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("claude execution finished")
	close(in.finishedChan)
}

func (in *Claude) Configure(consoleURL, consoleToken, deployToken string) error {
	mcp := NewMCPConfigBuilder()
	mcp.
		AddServer("plural", "mcpserver").
		Env("PLRL_CONSOLE_TOKEN", consoleToken).
		Env("PLRL_CONSOLE_URL", consoleURL).
		Done()
	if err := mcp.WriteToFile(filepath.Join(in.dir, ".mcp.json")); err != nil {
		return err
	}

	settings := NewSettingsBuilder()
	if in.run.Mode == console.AgentRunModeAnalyze {
		settings.AllowTools(
			"Read",
			"Grep",
			"Glob",
			"Bash(ls:*)",
			"Bash(cd:*)",
			"Bash(pwd)",
			"Bash(git status)",
			"Bash(git diff:*)",
			"Bash(head:*)",
			"Bash(tail:*)",
			"Bash(cat:*)",
			"Bash(grep:*)",
			"Bash(find:*)",
			"WebFetch",
			"mcp__plural__updateAgentRunAnalysis").
			DenyTools("Edit", "Write", "Bash(rm:*)", "Bash(sudo:*)")
	} else {
		settings.AllowTools(
			"Read",
			"Write",
			"Edit",
			"MultiEdit",
			"Bash",
			"WebFetch",
			"mcp__plural__agentPullRequest",
			"mcp__plural__createBranch",
			"mcp__plural__fetchAgentRunTodos",
			"mcp__plural__updateAgentRunTodos")
	}
	return settings.WriteToFile(filepath.Join(in.configPath(), "settings.local.json"))
}

func (in *Claude) configPath() string {
	return path.Join(in.dir, ".claude")
}

func (in *Claude) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func (in *Claude) ensure() error {
	if len(in.dir) == 0 {
		return fmt.Errorf("work directory is not set")
	}

	if len(in.repositoryDir) == 0 {
		return fmt.Errorf("repository directory is not set")
	}

	if in.run == nil {
		return fmt.Errorf("agent run is not set")
	}

	return nil
}

func mapClaudeContentToAgentMessage(event *StreamEvent, toolUseCache map[string]ContentMsg) *console.AgentMessageAttributes {
	msg := &console.AgentMessageAttributes{
		Role: mapRole(event.Message.Role),
	}

	var builder strings.Builder
	for _, c := range event.Message.Content {
		klog.V(log.LogLevelExtended).InfoS("claude content", "type", c.Type, "text", c.Text)

		switch c.Type {
		case "tool_use":
			// Cache tool name for later use in tool_result
			if c.ID != "" {
				toolUseCache[c.ID] = c
			}
		case "tool_result":
			output := ""
			if c.Content != nil {
				switch o := c.Content.(type) {
				case string:
					output = o
				default:
					if outputJSON, err := json.Marshal(o); err == nil {
						output = string(outputJSON)
					}
				}
			}
			toolUseContent, exists := toolUseCache[c.ToolUseID]
			if !exists {
				toolUseContent.Name = c.ToolUseID
			}
			klog.V(log.LogLevelDebug).InfoS("claude tool result", "tool_use_id", c.ToolUseID, "name", toolUseContent.Name, "is_error", c.IsError, "output", output)

			state := console.AgentMessageToolStateCompleted
			if c.IsError {
				state = console.AgentMessageToolStateError
			}
			msg.Role = console.AiRoleAssistant // Agent run tool calls should be marked as assistant messages.
			msg.Metadata = &console.AgentMessageMetadataAttributes{
				Tool: &console.AgentMessageToolAttributes{
					Name:   lo.ToPtr(toolUseContent.Name),
					State:  lo.ToPtr(state),
					Output: lo.ToPtr(output),
				},
			}

			input, err := json.Marshal(toolUseContent.Input)
			if err == nil {
				msg.Metadata.Tool.Input = lo.ToPtr(string(input))
			}

			builder.WriteString("Called tool")
		case "text":
			builder.WriteString(c.Text)
		}
	}
	msg.Message = builder.String()

	// Empty messages are not valid
	if len(msg.Message) == 0 {
		return nil
	}

	// Map usage → Cost
	if event.Message.Usage != nil {
		total := float64(event.Message.Usage.InputTokens + event.Message.Usage.OutputTokens)
		input := float64(event.Message.Usage.InputTokens)
		output := float64(event.Message.Usage.OutputTokens)

		msg.Cost = &console.AgentMessageCostAttributes{
			Total: total,
			Tokens: &console.AgentMessageTokensAttributes{
				Input:  &input,
				Output: &output,
			},
		}
	}

	return msg
}

func mapRole(role string) console.AiRole {
	switch strings.ToLower(role) {
	case "assistant":
		return console.AiRoleAssistant
	case "system":
		return console.AiRoleSystem
	case "user":
		return console.AiRoleUser
	default:
		return console.AiRoleSystem // Default to system role for unknown roles.
	}
}
