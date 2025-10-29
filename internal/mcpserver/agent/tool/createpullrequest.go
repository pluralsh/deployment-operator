package tool

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pluralsh/console/go/client"

	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment"
	console "github.com/pluralsh/deployment-operator/pkg/client"
)

func (in *CreatePullRequest) Install(server *server.MCPServer) {
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
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("The head branch (source branch with changes)"),
			),
		),
		in.handler,
	)
}

func (in *CreatePullRequest) handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	attrs, err := in.fromRequest(request)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not map request to attributes: %v", err)), nil
	}

	pr, err := in.client.CreateAgentPullRequest(ctx, in.agentRunID, attrs)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create pull request: %v", err)), nil
	}

	return mcp.NewToolResultJSON(struct {
		Success        bool             `json:"success"`
		Message        string           `json:"message"`
		PullRequestId  string           `json:"pullRequestId"`
		PullRequestUrl string           `json:"pullRequestUrl"`
		Status         *client.PrStatus `json:"status"`
		Title          *string          `json:"title"`
		Creator        *string          `json:"creator"`
	}{
		Success:        true,
		Message:        fmt.Sprintf("successfully created pull request from %s to %s", attrs.Head, attrs.Base),
		PullRequestId:  pr.ID,
		PullRequestUrl: pr.URL,
		Status:         pr.Status,
		Title:          pr.Title,
		Creator:        pr.Creator,
	})
}

func (in *CreatePullRequest) fromRequest(request mcp.CallToolRequest) (result client.AgentPullRequestAttributes, err error) {
	if result.Title, err = request.RequireString("title"); err != nil {
		return
	}

	if result.Body, err = request.RequireString("body"); err != nil {
		return
	}

	if result.Head, err = request.RequireString("head"); err != nil {
		return
	}

	config, err := environment.Load()
	if err != nil {
		return
	}

	result.Base = config.BaseBranch

	return
}

func NewCreatePullRequest(client console.Client, agentRunID string) Tool {
	return &CreatePullRequest{
		ConsoleTool: ConsoleTool{
			name:        "agentPullRequest",
			description: "Create a pull request through the Plural console GraphQL API for agent-generated changes",
			client:      client,
			agentRunID:  agentRunID,
		},
	}
}
