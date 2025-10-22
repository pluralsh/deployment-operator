package main

import (
	"flag"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/mcpserver/agent/args"
	"github.com/pluralsh/deployment-operator/internal/mcpserver/agent"
	"github.com/pluralsh/deployment-operator/internal/mcpserver/agent/tool"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	dev = "0.0.0"
)

// Version of this binary
var Version = dev

func init() {
	defaultFlagSet := flag.CommandLine

	// Init klog
	klog.InitFlags(defaultFlagSet)
}

func main() {
	klog.V(log.LogLevelDefault).InfoS("starting plural mcp server", "version", Version)

	client := console.New(args.ConsoleURL(), args.ConsoleToken())
	server := agent.NewServer(
		client,
		agent.WithTools(),
		agent.WithVersion(Version),
		agent.WithTool(tool.NewCreatePullRequest(client, args.AgentRunID())),
		agent.WithTool(tool.NewUpdateTodos(client, args.AgentRunID())),
		agent.WithTool(tool.NewUpdateAnalysis(client, args.AgentRunID())),
	)

	if err := server.Start(); err != nil {
		klog.Fatalf("Plural Console MCP server error: %v, exiting", err)
	}
}
