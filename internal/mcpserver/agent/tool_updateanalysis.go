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

func (in *UpdateAnalysis) Install(server *server.MCPServer) {
	server.AddTool(
		mcp.NewTool(
			in.name,
			mcp.WithDescription(in.description),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("The title of the pull request"),
			),
			mcp.WithString("body",
				mcp.Required(),
				mcp.Description("The body/description of the pull request"),
			),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("The base branch (target branch, usually 'main')"),
			),
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("The head branch (source branch with changes)"),
			),
		),
		in.handler,
	)
}

func (in *UpdateAnalysis) handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	attrs, err := in.fromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not map request to attributes: %v", err)), nil
	}

	agentRun, err := in.client.UpdateAgentRunAnalysis(ctx, in.agentRunID, attrs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update analysis: %v", err)), nil
	}

	return mcp.NewToolResultJSON(struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: fmt.Sprintf("successfully updated analysis for agent run %s", agentRun.ID),
	})
}

func (in *UpdateAnalysis) fromRequest(request mcp.CallToolRequest) (result client.AgentAnalysisAttributes, err error) {
	if result.Summary, err = request.RequireString("summary"); err != nil {
		return
	}

	if result.Analysis, err = request.RequireString("analysis"); err != nil {
		return
	}

	// Bullets is an optional field
	bullets, err := request.RequireStringSlice("bullets")
	if err == nil {
		result.Bullets = lo.ToSlicePtr(bullets)
	}

	return
}

func NewUpdateAnalysis(client console.Client, agentRunID string) Tool {
	return &UpdateAnalysis{
		ConsoleTool: ConsoleTool{
			name:        "updateAgentRunAnalysis",
			description: "Update the analysis in the system to provide summary, detailed analysis and bullet points for a given agent run",
			client:      client,
			agentRunID:  agentRunID,
		},
	}
}
