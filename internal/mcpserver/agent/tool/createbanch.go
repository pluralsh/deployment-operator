package tool

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
)

func (in *CreateBranch) Install(server *server.MCPServer) {
	server.AddTool(
		mcp.NewTool(
			in.name,
			mcp.WithDescription(in.description),
			mcp.WithString("branchName",
				mcp.Required(),
				mcp.Description("The name of the branch to create"),
			),
			mcp.WithString("commitMessage",
				mcp.Required(),
				mcp.Description("The body/description of the pull request"),
			),
		),
		in.handler,
	)
}

func (in *CreateBranch) handler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := in.fromRequest(request); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not handle create branch request: %v", err)), nil
	}

	config, err := environment.Load()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("could not load environment: %v", err)), nil
	}

	repoDir := config.Dir

	cmd := exec.NewExecutable("git", exec.WithArgs([]string{"checkout", "-b", in.BranchName}), exec.WithDir(repoDir))
	if err := cmd.Run(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to checkout branch: %v", err)), nil
	}

	cmd = exec.NewExecutable("git", exec.WithArgs([]string{"commit", "-m", in.CommitMessage}), exec.WithDir(repoDir))
	if err := cmd.Run(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to commit changes: %v", err)), nil
	}

	cmd = exec.NewExecutable("git", exec.WithArgs([]string{"push", "--set-upstream", "origin", in.BranchName}), exec.WithDir(repoDir))
	if err := cmd.Run(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to push changes: %v", err)), nil
	}

	return mcp.NewToolResultJSON(struct {
		Success       bool   `json:"success"`
		CommitMessage string `json:"message"`
		BranchName    string `json:"branchName"`
	}{
		Success:       true,
		CommitMessage: in.CommitMessage,
		BranchName:    in.BranchName,
	})
}

func (in *CreateBranch) fromRequest(request mcp.CallToolRequest) (err error) {
	if in.BranchName, err = request.RequireString("branchName"); err != nil {
		return
	}

	if in.CommitMessage, err = request.RequireString("commitMessage"); err != nil {
		return
	}

	return
}

func NewCreateBranch(client console.Client, agentRunID string) Tool {
	return &CreateBranch{
		ConsoleTool: ConsoleTool{
			name:        "createBranch",
			description: "Creates a new branch and commits current changes to it. This should always be used before creating a pull request",
			client:      client,
			agentRunID:  agentRunID,
		},
	}
}
