package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	console "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/gemini/events"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// Gemini implements v1.Tool interface.
type Gemini struct {
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

func (in *Gemini) Run(ctx context.Context, options ...exec.Option) {
	go in.start(ctx, options...)
}

func (in *Gemini) start(ctx context.Context, options ...exec.Option) {
	in.executable = exec.NewExecutable(
		"gemini",
		append(
			options,
			exec.WithArgs(in.args()),
			exec.WithDir(in.Config.WorkDir),
			exec.WithEnv([]string{fmt.Sprintf("GEMINI_API_KEY=%s", in.apiKey)}),
			exec.WithTimeout(15*time.Minute),
		)...,
	)

	// Send the initial prompt as a message too
	if in.onMessage != nil {
		in.onMessage(&console.AgentMessageAttributes{Message: in.Config.Run.Prompt, Role: console.AiRoleUser})
	}

	err := in.executable.RunStream(ctx, func(line []byte) {
		klog.V(log.LogLevelTrace).InfoS("Gemini stream event", "line", string(line))

		// This is here to prevent unavoidable log lines being reported as errors.
		// TODO: Remove once https://github.com/google-gemini/gemini-cli/issues/15053 is fixed.
		trimmed := strings.TrimSpace(string(line))
		if !strings.HasPrefix(trimmed, "{") {
			klog.V(log.LogLevelDebug).InfoS("ignoring non-json Gemini stream line", "trimmed", trimmed)
			return
		}

		event := &events.EventBase{}
		if err := json.Unmarshal(line, event); err != nil {
			klog.ErrorS(err, "failed to unmarshal Gemini stream event", "line", line)
			in.Config.ErrorChan <- err
			return
		}

		if err := event.OnMessage(line, in.onMessage); err != nil {
			klog.ErrorS(err, "failed to process Gemini stream event", "line", string(line))
			in.Config.ErrorChan <- err
		}
	})
	if err != nil {
		klog.ErrorS(err, "Gemini execution failed")
		in.Config.ErrorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("Gemini execution finished")
	close(in.Config.FinishedChan)
}

func (in *Gemini) args() []string {
	if in.Config.Run.Mode == console.AgentRunModeWrite {
		return []string{
			"--approval-mode", "yolo",
			"--output-format", "stream-json",
			in.Config.Run.Prompt,
		}
	}

	return []string{
		"--output-format", "stream-json",
		in.Config.Run.Prompt,
	}
}

func (in *Gemini) Configure(consoleURL, consoleToken, deployToken string) error {
	if err := in.DefaultTool.ConfigureSystemPrompt(console.AgentRuntimeTypeGemini); err != nil {
		return err
	}

	input := &ConfigTemplateInput{
		ConsoleURL:    consoleURL,
		ConsoleToken:  consoleToken,
		DeployToken:   deployToken,
		RepositoryDir: in.Config.RepositoryDir,
		AgentRunID:    in.Config.Run.ID,
		AgentRunMode:  in.Config.Run.Mode,
	}

	if in.Config.Run.Runtime.Type == console.AgentRuntimeTypeGemini {
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

func (in *Gemini) settingsPath() string {
	return path.Join(in.Config.WorkDir, ".gemini", SettingsFileName)
}

func (in *Gemini) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func New(config v1.Config) v1.Tool {
	if len(config.WorkDir) == 0 {
		klog.Fatalln("working directory is not set")
	}

	if len(config.RepositoryDir) == 0 {
		klog.Fatalln("repository directory is not set")
	}

	if config.Run == nil {
		klog.Fatalln("agent run is not set")
	}

	return &Gemini{
		DefaultTool: v1.DefaultTool{Config: config},
		//dir:           config.WorkDir,
		//repositoryDir: config.RepositoryDir,
		//run:           config.Run,
		apiKey: helpers.GetEnv(controller.EnvGeminiAPIKey, ""),
		model:  DefaultModel(),
		//finishedChan:  config.FinishedChan,
		//errorChan:     config.ErrorChan,
		//startedChan:   make(chan struct{}),
	}
}
