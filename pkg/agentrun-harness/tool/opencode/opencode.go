package opencode

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/polly/algorithms"
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

func (in *Opencode) Configure(consoleURL string, deployToken string) error {
	input := &ConfigTemplateInput{
		ConsoleURL:    consoleURL,
		DeployToken:   deployToken,
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
		go in.runPrompt(ctx)
	}

	<-ctx.Done()
	if err := context.Cause(ctx); err != nil {
		in.errorChan <- err
		return
	}

	close(in.finishedChan)
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
	if err != nil {
		klog.V(log.LogLevelDefault).ErrorS(err, "opencode server stopped")
		in.contextCancel(err)
	}
}

func (in *Opencode) runPrompt(ctx context.Context) {
	client := opencode.NewClient(option.WithBaseURL(fmt.Sprintf("http://localhost:%s", in.port)))
	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Title: opencode.F("Plural Agent Run"),
	})
	if err != nil {
		in.contextCancel(err)
		return
	}

	klog.V(log.LogLevelExtended).InfoS("sending prompt", "prompt", in.run.Prompt)
	res, err := client.Session.Prompt(ctx, session.ID, opencode.SessionPromptParams{
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			opencode.TextPartInputParam{
				Text: opencode.F(in.run.Prompt),
				Type: opencode.F(opencode.TextPartInputTypeText),
			},
		}),
		System: opencode.F(systemPrompt),
		Agent:  opencode.F(in.agent()),
		Model: opencode.F(opencode.SessionPromptParamsModel{
			ModelID:    opencode.F(defaultModelID),
			ProviderID: opencode.F(defaultProviderID),
		}),
	})
	if err != nil {
		in.contextCancel(err)
		return
	}

	klog.V(log.LogLevelExtended).InfoS("opencode prompt response", "response", strings.Join(algorithms.Map(res.Parts, func(part opencode.Part) string {
		return part.Text
	}), " "))
	in.contextCancel(nil)
}

func (in *Opencode) agent() string {
	return lo.Ternary(in.run.Mode == console.AgentRunModeAnalyze, defaultAnalysisAgent, defaultWriteAgent)
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
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize opencode tool: %v", err)
	}

	return result
}
