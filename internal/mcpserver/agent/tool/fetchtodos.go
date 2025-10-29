package tool

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (in *GetAgentRunTodosTool) Install(server *server.MCPServer) {
	server.AddTool(
		mcp.NewTool(
			in.name,
			mcp.WithDescription(in.description),
		),
		in.handler,
	)
}

func (in *GetAgentRunTodosTool) handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runId, err := request.RequireString("runId")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get runId: %v", err)), nil
	}

	cachedAgentRun, exists := GetAgentRun(runId)

	// if the agentRun with the given runId is not in the cache, fetch it from the API and cache it
	if !exists {
		fragment, err := in.client.GetAgentRun(ctx, runId)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get agent run: %v", err)), nil
		}
		SetAgentRun(runId, fragment)
		cachedAgentRun = fragment
	}

	// fetch the agentRun from the cache and return the todos fragment, exists = GetAgentRun(runId)
	// we need to convert the todos in order to return the correct format for the MCP server
	todos := make([]map[string]interface{}, 0)
	if cachedAgentRun.Todos != nil {
		for _, todo := range cachedAgentRun.Todos {
			todoMap := map[string]interface{}{
				"title":       todo.Title,
				"done":        todo.Done,
				"description": todo.Description,
			}
			todos = append(todos, todoMap)
		}
	}

	return mcp.NewToolResultJSON(struct {
		Success bool                     `json:"success"`
		Message string                   `json:"message"`
		RunId   string                   `json:"runId"`
		Todos   []map[string]interface{} `json:"todos"`
		Source  string                   `json:"source"`
	}{
		Success: true,
		Message: fmt.Sprintf("Successfully fetched todos for agent run %s", runId),
		RunId:   runId,
		Todos:   todos,
		Source:  "cache",
	})
}
