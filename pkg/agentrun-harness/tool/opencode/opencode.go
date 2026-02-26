package opencode

import (
	"context"
	"errors"
	"fmt"
	"path"

	console "github.com/pluralsh/console/go/client"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
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
	maxRestarts := 2
	restarts := 0

	for {
		if restarts >= maxRestarts {
			in.Config.ErrorChan <- fmt.Errorf("failed to process prompt after %d attempts", maxRestarts)
			return
		}

		if err := in.server.Start(ctx, options...); err != nil {
			klog.V(log.LogLevelDefault).ErrorS(err, "failed to start opencode server")
			in.Config.ErrorChan <- err
			return
		}

		messageChan, listenErrChan := in.server.Listen(ctx)

		// Send the initial prompt as a message too
		in.onMessage(&console.AgentMessageAttributes{Message: in.Config.Run.Prompt, Role: console.AiRoleUser})
		promptDone, promptErr := in.server.Prompt(ctx, in.Config.Run.Prompt)

	restart:
		for {
			select {
			case <-ctx.Done():
				in.server.Stop()
				close(in.Config.FinishedChan)
				return
			case <-promptDone:
				in.server.Stop()
				close(in.Config.FinishedChan)
				return
			case msg := <-messageChan:
				klog.V(log.LogLevelDefault).InfoS("message received", "message", msg)
				in.onMessage(msg.Message)
			case err := <-listenErrChan:
				in.Config.ErrorChan <- err
				return
			case err := <-promptErr:
				if errors.Is(err, context.DeadlineExceeded) {
					in.server.Stop()
					restarts++
					klog.V(log.LogLevelDefault).InfoS("prompt timed out, restarting server", "restart", restarts, "maxAttempts", maxRestarts)
					break restart
				}

				in.Config.ErrorChan <- err
				return
			}
		}
	}
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
		//run:           config.Run,
		model:    DefaultModel(),
		provider: DefaultProvider(config.Run.IsProxyEnabled()),
		//dir:           config.WorkDir,
		//repositoryDir: config.RepositoryDir,
		//finishedChan:  config.FinishedChan,
		//errorChan:     config.ErrorChan,
		//startedChan:   make(chan struct{}),
		port:   defaultOpenCodePort,
		client: opencode.NewClient(option.WithBaseURL(fmt.Sprintf("http://localhost:%s", defaultOpenCodePort))),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize opencode tool: %v", err)
	}

	result.server = NewServer(defaultOpenCodePort, result.configFilePath(), config.RepositoryDir, result.model, result.provider, config.Run.Mode)
	return result
}
