package tool

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/pluralsh/console/go/client"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

// Tool is an MCP tool that can be installed on the MCP server
type Tool interface {
	Name() string
	Install(server *server.MCPServer)
}

type ConsoleTool struct {
	// name is the name of the tool to register
	name string

	// description is the description of the tool
	description string

	// client is the Plural Console client
	client console.Client

	// agentRunID is the ID of the agent run that is being processed
	agentRunID string
}

func (t *ConsoleTool) Name() string {
	return t.name
}

// CreatePullRequest is an MCP tool that creates a pull request for a given agent run
type CreatePullRequest struct {
	ConsoleTool
}

// UpdateTodos is an MCP tool that updates the todos for a given agent run
type UpdateTodos struct {
	ConsoleTool
}

// UpdateTodosInputSchema is the input schema for the UpdateTodos tool
type UpdateTodosInputSchema struct {
	Todos []client.AgentTodoAttributes `json:"todos"`
}

// UpdateAnalysis is an MCP tool that updates the analysis for a given agent run
type UpdateAnalysis struct {
	ConsoleTool
}

// GetAgentRunTodos is an MCP tool that gets the todos for a given agent run
type GetAgentRunTodosTool struct {
	ConsoleTool
}

// cachedAgentRun holds the current agent run fragment shared across tools
var cachedAgentRun *client.AgentRunFragment
