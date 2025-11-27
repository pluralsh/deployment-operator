package claude

import (
	"context"
	"fmt"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"k8s.io/klog/v2"
)

func New(config v1.Config) v1.Tool {
	result := &Claude{
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		run:           config.Run,
		token:         helpers.GetEnv(controller.EnvClaudeToken, ""),
		model:         DefaultModel(),
	}

	if err := result.ensure(); err != nil {
		klog.Fatalf("failed to initialize claude tool: %v", err)
	}

	return result
}

func (in *Claude) Run(ctx context.Context, options ...exec.Option) {
	in.executable = exec.NewExecutable(
		"claude",
		append(
			options,
			exec.WithArgs([]string{"--add-dir", in.repositoryDir, "-p", in.run.Prompt, "--output-format", "stream-json", "--verbose"}),
			exec.WithDir(in.dir),
			exec.WithEnv([]string{fmt.Sprintf("ANTHROPIC_API_KEY=%s", in.token)}),
		)...,
	)

	err := in.executable.RunStream(ctx, func(line []byte) {
		fmt.Println("STREAM:", string(line))
	})
	if err != nil {
		klog.ErrorS(err, "stream execution failed")
	}
}

func (in *Claude) Configure(consoleURL, deployToken, consoleToken string) error {
	return nil
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
