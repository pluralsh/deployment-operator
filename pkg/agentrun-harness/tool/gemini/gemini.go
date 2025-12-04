package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	agentrun "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/gemini/events"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"k8s.io/klog/v2"
)

const (
	analyzeModeContextFileName = "ANALYZE.md"
	writeModeContextFileName   = "WRITE.md"
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

	// executable is the Gemini executable used to call CLI.
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
		klog.V(log.LogLevelTrace).InfoS("Gemini stream event", "line", string(line))

		event := &events.EventBase{}
		if err := json.Unmarshal(line, event); err != nil {
			klog.ErrorS(err, "failed to unmarshal Gemini stream event", "line", string(line))
			in.errorChan <- err
			return
		}

		if err := event.OnMessage(line, in.onMessage); err != nil {
			klog.ErrorS(err, "failed to process Gemini stream event", "line", string(line))
			in.errorChan <- err
		}
	})
	if err != nil {
		klog.ErrorS(err, "Gemini execution failed")
		in.errorChan <- err
		return
	}
	klog.V(log.LogLevelExtended).InfoS("Gemini execution finished")
	close(in.finishedChan)
}

func (in *Gemini) contextFileName() string {
	if in.run == nil {
		return analyzeModeContextFileName
	}

	switch in.run.Mode {
	case console.AgentRunModeWrite:
		return writeModeContextFileName
	case console.AgentRunModeAnalyze:
		return analyzeModeContextFileName
	default:
		return analyzeModeContextFileName
	}
}

func (in *Gemini) Configure(consoleURL, consoleToken, _ string) error {
	input := &ConfigTemplateInput{
		ConsoleURL:      consoleURL,
		ConsoleToken:    consoleToken,
		RepositoryDir:   in.repositoryDir,
		AgentRunID:      in.run.ID,
		ContextFileName: in.contextFileName(),
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
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		run:           config.Run,
		apiKey:        helpers.GetEnv(controller.EnvGeminiAPIKey, ""),
		model:         DefaultModel(),
		finishedChan:  config.FinishedChan,
		errorChan:     config.ErrorChan,
		startedChan:   make(chan struct{}),
	}
}
