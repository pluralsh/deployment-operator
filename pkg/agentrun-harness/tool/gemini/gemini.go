package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	agentrun "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
)

// Gemini implements v1.Tool interface.
type Gemini struct {
	// dir is a working directory used to run Gemini.
	dir string

	// repositoryDir is a directory where the cloned repository is located.
	repositoryDir string

	// run is the agent run that is being processed.
	run *agentrun.AgentRun

	// onMessage is a callback called when a new message is received.
	onMessage func(message *console.AgentMessageAttributes)

	// executable is the claude executable used to call CLI.
	executable exec.Executable

	// apiKey used to authenticate with the API.
	apiKey string

	// model used to generate code.
	model Model

	// errorChan is a channel that returns an error if the tool failed.
	errorChan chan error

	// finishedChan is a channel that gets closed when the tool is finished.
	finishedChan chan struct{}

	// startedChan is a channel that gets closed when the Gemini server is started.
	startedChan chan struct{}
}

func (in *Gemini) ensure() error {
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

func (in *Gemini) settingsPath() string {
	return path.Join(in.dir, ".gemini", SettingsFileName)
}

func (in *Gemini) Run(ctx context.Context, options ...exec.Option) {
	in.executable = exec.NewExecutable(
		"gemini",
		append(
			options,
			exec.WithArgs([]string{
				"--include-directories", in.repositoryDir,
				"--output-format", "stream-json",
				in.run.Prompt,
			}),
			exec.WithDir(in.dir),
			exec.WithEnv([]string{fmt.Sprintf("GEMINI_API_KEY=%s", in.apiKey)}),
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
			klog.ErrorS(err, "failed to unmarshal Gemini stream event", "line", string(line))
			in.errorChan <- err
			return
		}

		if event.Type == "assistant" && event.Content != nil {
			msg := mapClaudeContentToAgentMessage(event)
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

func (in *Gemini) Configure(consoleURL, consoleToken, deployToken string) error {
	input := &ConfigTemplateInput{
		ConsoleURL:    consoleURL,
		ConsoleToken:  consoleToken,
		AgentRunID:    in.run.ID,
		RepositoryDir: in.repositoryDir,
	}

	if in.run.Runtime.Type == console.AgentRuntimeTypeGemini {
		input.Model = DefaultModel()
	}

	_, content, err := settings(input)
	if err != nil {
		return err
	}

	if err = helpers.File().Create(in.settingsPath(), content); err != nil {
		return fmt.Errorf("failed configuring Gemini settings file %q: %w", SettingsFileName, err)
	}

	klog.V(log.LogLevelExtended).InfoS("Gemini configured", "settings", in.settingsPath())
	return nil
}

func (in *Gemini) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func New(config v1.Config) v1.Tool {
	result := &Gemini{
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		run:           config.Run,
		apiKey:        helpers.GetEnv(controller.EnvGeminiAPIKey, ""),
		model:         DefaultModel(),
		finishedChan:  config.FinishedChan,
		errorChan:     config.ErrorChan,
		startedChan:   make(chan struct{}),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize Gemini tool: %v", err)
	}

	return result
}

func mapClaudeContentToAgentMessage(event *StreamEvent) *console.AgentMessageAttributes {
	klog.V(log.LogLevelExtended).InfoS("Gemini event", "type", event.Type, "content", event.Content)

	msg := &console.AgentMessageAttributes{
		Role:    mapRole(event),
		Message: lo.FromPtr(event.Content),
	}

	// TODO

	// Empty messages are not valid.
	if len(msg.Message) > 0 {
		return nil
	}

	if event.Stats != nil {
		total := float64(event.Stats.TotalTokens)
		input := float64(event.Stats.InputTokens)
		output := float64(event.Stats.OutputTokens)

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

func mapRole(event *StreamEvent) console.AiRole {
	if event == nil {
		return console.AiRoleUser // TODO
	}

	switch strings.ToLower(*event.Role) {
	case "assistant":
		return console.AiRoleAssistant
	case "system":
		return console.AiRoleSystem // TODO: Verify.
	case "user":
		return console.AiRoleUser
	default:
		return console.AiRoleUser // TODO: System?
	}
}
