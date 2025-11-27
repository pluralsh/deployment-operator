package main

import (
	"context"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/claude"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
)

func main() {

	r := claude.New(v1.Config{
		WorkDir:       ".",
		RepositoryDir: ".",
		FinishedChan:  nil,
		ErrorChan:     nil,
		Run: &agentrunv1.AgentRun{
			Prompt: "Explain the purpose of these files",
		},
	})
	r.Run(context.Background())
}
