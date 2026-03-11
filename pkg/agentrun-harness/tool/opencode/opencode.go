package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/sst/opencode-sdk-go"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *Opencode) Run(ctx context.Context, options ...exec.Option) {
	go in.start(ctx, options...)
}

func (in *Opencode) Configure(consoleURL, consoleToken, deployToken string) error {
	if err := in.ConfigureSystemPrompt(console.AgentRuntimeTypeOpencode); err != nil {
		return err
	}

	input := &ConfigTemplateInput{
		ConsoleURL:   consoleURL,
		ConsoleToken: consoleToken,
		DeployToken:  deployToken,
		AgentRunID:   in.Config.Run.ID,
		Provider:     in.provider,
		Endpoint:     helpers.GetEnv(controller.EnvOpenCodeEndpoint, in.provider.Endpoint()),
		Model:        in.model,
		Token:        helpers.GetEnv(controller.EnvOpenCodeToken, ""),
		Mode:         in.Config.Run.Mode,
	}

	_, content, err := configTemplate(input)
	if err != nil {
		return err
	}

	if err = helpers.File().Create(in.configFilePath(), content, 0644); err != nil {
		return fmt.Errorf("failed configuring opencode config file %q: %w", ConfigFileName, err)
	}

	klog.V(log.LogLevelExtended).InfoS("opencode configured", "configFile", in.configFilePath())
	return nil
}

func (in *Opencode) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}

func (in *Opencode) start(ctx context.Context, options ...exec.Option) {
	configFilePath, err := filepath.Abs(in.configFilePath())
	if err != nil {
		in.Config.ErrorChan <- err
		return
	}

	in.executable = exec.NewExecutable(
		"opencode",
		append(
			options,
			exec.WithEnv([]string{fmt.Sprintf("OPENCODE_CONFIG=%s", configFilePath)}),
			exec.WithArgs(in.args()),
			exec.WithDir(in.Config.RepositoryDir),
			exec.WithTimeout(in.timeout),
		)...,
	)

	// Send the initial prompt as a message too
	if in.onMessage != nil {
		in.onMessage(&console.AgentMessageAttributes{Message: in.Config.Run.Prompt, Role: console.AiRoleUser})
	}

	events := make(map[string]*Event)
	err = in.executable.RunStream(ctx, func(line []byte) {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || !bytes.HasPrefix(trimmed, []byte("{")) {
			return
		}

		event := &opencode.EventListResponse{}
		if err := json.Unmarshal(trimmed, event); err != nil {
			klog.V(log.LogLevelDebug).ErrorS(err, "failed to unmarshal opencode stream event", "line", string(trimmed))
			return
		}

		id := in.getID(*event)
		if len(id) == 0 {
			return
		}

		aggregated, exists := events[id]
		if !exists {
			aggregated = &Event{}
		}

		aggregated.FromEventResponse(*event)
		events[id] = aggregated

		if aggregated.Done {
			aggregated.Sanitize()
			if in.onMessage != nil {
				in.onMessage(aggregated.Message)
			}
			delete(events, id)
		}
	})
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "opencode execution failed")
		in.Config.ErrorChan <- err
		return
	}

	klog.V(log.LogLevelExtended).InfoS("opencode execution finished")
	close(in.Config.FinishedChan)
}

func (in *Opencode) getID(e opencode.EventListResponse) string {
	switch e.Type {
	case opencode.EventListResponseTypeMessageUpdated:
		return e.Properties.(opencode.EventListResponseEventMessageUpdatedProperties).Info.ID
	case opencode.EventListResponseTypeMessagePartUpdated:
		return e.Properties.(opencode.EventListResponseEventMessagePartUpdatedProperties).Part.MessageID
	default:
		return ""
	}
}

func (in *Opencode) args() []string {
	model := lo.Ternary(in.Config.Run.IsProxyEnabled(), fmt.Sprintf("%s/%s", in.provider, in.model), string(in.model))

	return []string{
		"run",
		"--format", "json",
		"--agent", in.agent(),
		"--model", model,
		in.Config.Run.Prompt,
	}
}

func (in *Opencode) agent() string {
	if in.Config.Run.Mode == console.AgentRunModeAnalyze {
		return defaultAnalysisAgent
	}

	return defaultWriteAgent
}

func (in *Opencode) configFilePath() string {
	return path.Join(in.Config.WorkDir, ".opencode", ConfigFileName)
}

func (in *Opencode) ensure() error {
	if len(in.Config.WorkDir) == 0 {
		return fmt.Errorf("work directory is not set")
	}

	if len(in.Config.RepositoryDir) == 0 {
		return fmt.Errorf("repository directory is not set")
	}

	if in.Config.FinishedChan == nil {
		return fmt.Errorf("finished channel is not set")
	}

	if in.Config.ErrorChan == nil {
		return fmt.Errorf("error channel is not set")
	}

	if in.Config.Run == nil {
		return fmt.Errorf("agent run is not set")
	}

	return nil
}

func New(config v1.Config) v1.Tool {
	result := &Opencode{
		DefaultTool: v1.DefaultTool{Config: config},
		model:       DefaultModel(),
		provider:    DefaultProvider(config.Run.IsProxyEnabled()),
		timeout:     30 * time.Minute,
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize opencode tool: %v", err)
	}

	return result
}
