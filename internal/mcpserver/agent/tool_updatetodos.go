package agent

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

func (in *UpdateTodos) Install(server *server.MCPServer) {
	server.AddTool(
		mcp.NewTool(
			in.name,
			mcp.WithDescription(in.description),
			mcp.WithInputSchema[UpdateTodosInputSchema](),
		),
		in.handler,
	)
}

func (in *UpdateTodos) handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	attrs, err := in.fromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not map request to attributes: %v", err)), nil
	}

	agentRun, err := in.client.UpdateAgentRunTodos(ctx, in.agentRunID, attrs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update todos: %v", err)), nil
	}

	return mcp.NewToolResultJSON(struct {
		Success bool                        `json:"success"`
		Message string                      `json:"message"`
		Done    []*client.AgentTodoFragment `json:"done"`
		Todo    []*client.AgentTodoFragment `json:"todo"`
	}{
		Success: true,
		Message: fmt.Sprintf("successfully updated todos for agent run %s", agentRun.ID),
		Done: lo.Filter(agentRun.Todos, func(item *client.AgentTodoFragment, index int) bool {
			return item != nil && item.Done != nil && *item.Done
		}),
		Todo: lo.Filter(agentRun.Todos, func(item *client.AgentTodoFragment, index int) bool {
			return item != nil && item.Done != nil && !*item.Done
		}),
	})
}

func (in *UpdateTodos) fromRequest(request mcp.CallToolRequest) ([]*client.AgentTodoAttributes, error) {
	todosInterface, ok := request.GetArguments()["todos"]
	if !ok {
		return nil, fmt.Errorf("missing todos argument")
	}

	todos, ok := todosInterface.([]client.AgentTodoAttributes)
	if !ok {
		return nil, fmt.Errorf("todos argument is not a list")
	}

	return lo.ToSlicePtr(todos), nil
}

func NewUpdateTodos(client console.Client, agentRunID string) Tool {
	return &UpdateTodos{
		ConsoleTool: ConsoleTool{
			name:        "updateAgentRunTodos",
			description: "Update the todo checklist progress in the system to keep track of what needs to be done for a given agent run",
			client:      client,
			agentRunID:  agentRunID,
		},
	}
}
