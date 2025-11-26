package claude

import (
	"context"

	console "github.com/pluralsh/console/go/client"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

func New(config v1.Config) v1.Tool {
	result := &Claude{
		dir:           config.WorkDir,
		repositoryDir: config.RepositoryDir,
		run:           config.Run,
	}

	return result
}

func (in *Claude) Run(ctx context.Context, options ...exec.Option) {

}

func (in *Claude) Configure(consoleURL, deployToken, consoleToken string) error {
	return nil
}

func (in *Claude) OnMessage(f func(message *console.AgentMessageAttributes)) {
	in.onMessage = f
}
