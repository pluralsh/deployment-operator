package main

import (
	"context"
	"flag"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/claude"
	v1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "5") // or whatever level LogLevelExtended has
	flag.Parse()

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
