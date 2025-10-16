package agent

import (
	"github.com/mark3labs/mcp-go/server"
	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// Start starts the MCP server with stdio transport
func (in *Server) Start() error {
	klog.V(log.LogLevelDefault).Info("started plural console mcp server", "version", in.version)
	return server.ServeStdio(in.server)
}

func (in *Server) init() *Server {
	in.server = server.NewMCPServer(
		in.name,
		in.version,
		server.WithToolCapabilities(in.toolsEnabled),
	)

	for _, tool := range in.tools {
		tool.Install(in.server)
		klog.V(log.LogLevelDefault).InfoS("registered tool with mcp server", "tool", tool.Name())
	}

	return in
}

// NewServer creates a new MCP server instance
func NewServer(client console.Client, options ...Option) *Server {
	mcpServer := &Server{
		name:    "Plural Console MCP Server",
		version: "0.0.0-dev",
		client:  client,
	}

	for _, option := range options {
		option(mcpServer)
	}

	return mcpServer.init()
}
