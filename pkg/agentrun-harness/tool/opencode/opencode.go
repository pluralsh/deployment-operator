package opencode

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"dario.cat/mergo"
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *Opencode) Run(ctx context.Context, options ...exec.Option) {
	internalCtx, cancel := context.WithCancelCause(ctx)
	in.contextCancel = cancel

	go in.waitForServer(internalCtx)
	go in.runServer(internalCtx, options...)
}

func (in *Opencode) Configure(consoleURL, consoleToken, deployToken string) error {
	input := &ConfigTemplateInput{
		ConsoleURL:    consoleURL,
		ConsoleToken:  consoleToken,
		DeployToken:   deployToken,
		AgentRunID:    in.run.ID,
		ModelID:       defaultModelID,
		ModelName:     defaultModelName,
		ProviderID:    defaultProviderID,
		ProviderName:  defaultProviderName,
		AnalysisAgent: defaultAnalysisAgent,
		WriteAgent:    defaultWriteAgent,
	}
	_, content, err := configTemplate(input)
	if err != nil {
		return err
	}

	if err = helpers.File().Create(in.configFilePath(), content); err != nil {
		return fmt.Errorf("failed configuring opencode config file %q: %w", ConfigFileName, err)
	}

	klog.V(log.LogLevelExtended).InfoS("opencode configured", "configFile", in.configFilePath())
	return nil
}

func (in *Opencode) waitForServer(ctx context.Context) {
	klog.V(log.LogLevelExtended).InfoS("waiting for opencode server to start")
	select {
	case <-ctx.Done():
		if err := context.Cause(ctx); err != nil {
			in.errorChan <- err
			return
		}

		close(in.finishedChan)
		return
	case <-in.startedChan:
		go in.runEventListener(ctx)
		go in.runPrompt(ctx)
	}

	<-ctx.Done()
	if err := context.Cause(ctx); err != nil && !errors.Is(err, context.Canceled) {
		in.errorChan <- err
		return
	}

	close(in.finishedChan)
}

type Message struct {
	ID        string
	Role      string
	Mode      string
	Model     string
	Provider  string
	MType     string
	Tool      string
	Files     []string
	State     opencode.ToolPartState
	Completed bool
}

func (in *Opencode) runEventListener(ctx context.Context) {
	messages := make(map[string]Message)
	stream := in.client.Event.ListStreaming(ctx, opencode.EventListParams{})
	for stream.Next() {
		data := stream.Current()
		switch data.Type {
		case opencode.EventListResponseTypeMessageUpdated:
			properties := data.Properties.(opencode.EventListResponseEventMessageUpdatedProperties)
			messages = in.updateMessage(messages, properties.Info.ID, Message{
				ID:       properties.Info.ID,
				Role:     string(properties.Info.Role),
				Mode:     properties.Info.Mode,
				Model:    properties.Info.ModelID,
				Provider: properties.Info.ProviderID,
			})
		case opencode.EventListResponseTypeMessagePartUpdated:
			properties := data.Properties.(opencode.EventListResponseEventMessagePartUpdatedProperties)
			files, _ := properties.Part.Files.([]string)
			state, _ := properties.Part.State.(opencode.ToolPartState)
			messages = in.updateMessage(messages, properties.Part.MessageID, Message{
				ID:        properties.Part.MessageID,
				MType:     string(properties.Part.Type),
				Tool:      properties.Part.Tool,
				Files:     files,
				State:     state,
				Completed: properties.Part.Type == "step-finish",
			})
		case opencode.EventListResponseTypeFileEdited:
			properties := data.Properties.(opencode.EventListResponseEventFileEditedProperties)
			klog.InfoS("file edited", "file", properties.File)
		case opencode.EventListResponseTypeFileWatcherUpdated:
			properties := data.Properties.(opencode.EventListResponseEventFileWatcherUpdatedProperties)
			klog.InfoS("file watcher updated", "file", properties.File, "event", properties.Event)
		case opencode.EventListResponseTypePermissionReplied:
			properties := data.Properties.(opencode.EventListResponseEventPermissionRepliedProperties)
			klog.InfoS("permission replied", "permission", properties.PermissionID, "response", properties.Response)
		case opencode.EventListResponseTypePermissionUpdated:
			properties := data.Properties.(opencode.EventListResponseEventPermissionUpdated)
			klog.InfoS("permission updated",
				"title", properties.Properties.Title,
				"type", properties.Properties.Type,
				"message", properties.Properties.MessageID,
				"session", properties.Properties.SessionID,
				"metadata", properties.Properties.Metadata,
			)
		case opencode.EventListResponseTypeServerConnected:
			properties, ok := data.Properties.(opencode.EventListResponseEventServerConnected)
			if !ok {
				continue
			}
			klog.InfoS("server connected", "properties", properties.Properties)
		}

		for id, msg := range messages {
			if msg.Completed {
				toPrint := msg
				klog.InfoS("new message", "msg", toPrint)
				delete(messages, id)
			}
		}
	}

	if err := stream.Err(); err != nil {
		in.contextCancel(err)
	}

	klog.V(log.LogLevelDefault).InfoS("opencode event listener stopped")
}

func (in *Opencode) updateMessage(m map[string]Message, id string, msg Message) map[string]Message {
	existing, ok := m[id]
	if !ok {
		m[id] = msg
		return m
	}

	t := existing.MType
	_ = mergo.Merge(&existing, msg, mergo.WithOverride)

	if msg.MType != "step-finish" {
		t = msg.MType
	}

	existing.MType = t
	m[id] = existing

	return m
}

func (in *Opencode) runServer(ctx context.Context, options ...exec.Option) {
	configFilePath, err := filepath.Abs(in.configFilePath())
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to get absolute path to opencode config file")
		in.contextCancel(err)
		return
	}

	klog.V(log.LogLevelExtended).InfoS("starting opencode server", "config", configFilePath)
	executable := exec.NewExecutable(
		"opencode",
		append(
			options,
			exec.WithEnv([]string{fmt.Sprintf("OPENCODE_CONFIG=%s", configFilePath)}),
			exec.WithArgs([]string{"serve", "--port", in.port}),
			exec.WithDir(in.repositoryDir),
		)...,
	)

	waitFn, err := executable.Start(ctx)
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "failed to start opencode server")
		in.contextCancel(err)
		return
	}

	close(in.startedChan)

	err = waitFn()
	if err != nil && !errors.Is(ctx.Err(), context.Canceled) {
		klog.V(log.LogLevelDefault).ErrorS(err, "opencode server exited with error")
		in.contextCancel(err)
	}

	klog.V(log.LogLevelDefault).InfoS("opencode server stopped")
}

func (in *Opencode) runPrompt(ctx context.Context) {
	session, err := in.client.Session.New(ctx, opencode.SessionNewParams{
		Title: opencode.F("Plural Agent Run"),
	})
	if err != nil {
		in.contextCancel(err)
		return
	}

	// TODO: remove after testing
	in.run.Prompt = "Create or update main README.md file based on the contents of repository and then create a pull request with the proposed changes for further review."
	in.sessionID = session.ID

	maxRetries := 3
	retried := 0
	requestTimeout := 2 * time.Minute

	for {
		internalCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		select {
		case <-ctx.Done():
			cancel()
			return
		default:
			if retried >= maxRetries {
				in.contextCancel(fmt.Errorf("could not send prompt after %d retries", maxRetries))
				cancel()
				return
			}

			klog.V(log.LogLevelExtended).InfoS("sending prompt", "prompt", in.run.Prompt)
			res, err := in.client.Session.Prompt(internalCtx, session.ID, opencode.SessionPromptParams{
				Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
					opencode.TextPartInputParam{
						Text: opencode.F(in.run.Prompt),
						Type: opencode.F(opencode.TextPartInputTypeText),
					},
				}),
				System: opencode.F(in.systemPrompt()),
				Agent:  opencode.F(in.agent()),
				Model: opencode.F(opencode.SessionPromptParamsModel{
					ModelID:    opencode.F(defaultModelID),
					ProviderID: opencode.F(defaultProviderID),
				}),
			})
			if errors.Is(internalCtx.Err(), context.DeadlineExceeded) {
				res, err := in.client.Session.Abort(ctx, in.sessionID, opencode.SessionAbortParams{})
				if err != nil {
					cancel()
					in.contextCancel(err)
					return
				}

				klog.InfoS("prompt aborted, retrying", "response", res)
				retried++
				continue
			}

			if err != nil {
				cancel()
				in.contextCancel(err)
				return
			}

			klog.V(log.LogLevelDefault).InfoS("prompt sent successfully", "response", res)
			cancel()
			in.contextCancel(nil)
			return
		}
	}
}

func (in *Opencode) agent() string {
	// TODO: fix logic after testing
	return lo.Ternary(in.run.Mode == console.AgentRunModeAnalyze, defaultWriteAgent, defaultWriteAgent)
}

func (in *Opencode) systemPrompt() string {
	// TODO: fix logic after testing
	return lo.Ternary(in.run.Mode == console.AgentRunModeAnalyze, systemPromptWriter, systemPromptWriter)
}

func (in *Opencode) configFilePath() string {
	return path.Join(in.dir, ".config", ConfigFileName)
}

func (in *Opencode) ensure() error {
	if len(in.dir) == 0 {
		return fmt.Errorf("work directory is not set")
	}

	if len(in.repositoryDir) == 0 {
		return fmt.Errorf("repository directory is not set")
	}

	if in.finishedChan == nil {
		return fmt.Errorf("finished channel is not set")
	}

	if in.errorChan == nil {
		return fmt.Errorf("error channel is not set")
	}

	if in.run == nil {
		return fmt.Errorf("agent run is not set")
	}

	return nil
}

func New(config v1.Config) v1.Tool {
	result := &Opencode{
		run:           config.Run,
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		finishedChan:  config.FinishedChan,
		errorChan:     config.ErrorChan,
		startedChan:   make(chan struct{}),
		port:          defaultOpenCodePort,
		client:        opencode.NewClient(option.WithBaseURL(fmt.Sprintf("http://localhost:%s", defaultOpenCodePort))),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize opencode tool: %v", err)
	}

	return result
}
