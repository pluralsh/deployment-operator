package agentprserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	console "github.com/pluralsh/deployment-operator/pkg/client"
)

// PluralCredentials holds the Plural console credentials
type PluralCredentials struct {
	AccessToken string
	ConsoleURL  string
}

// MCPServer wraps the mcp server with Plural credentials
type MCPServer struct {
	server *server.MCPServer
	client console.Client
	creds  *PluralCredentials
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(creds *PluralCredentials) *MCPServer {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Plural Console GraphQL MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	mcpServer := &MCPServer{
		server: s,
		creds:  creds,
		client: console.New(creds.ConsoleURL, creds.AccessToken),
	}

	// Register tools
	mcpServer.registerTools()

	return mcpServer
}

// LoadPluralCredentials loads and validates Plural credentials from environment variables
func LoadPluralCredentials() (*PluralCredentials, error) {
	accessToken := os.Getenv(controller.EnvConsoleToken)
	if accessToken == "" {
		return nil, fmt.Errorf("%s environment variable is required", controller.EnvConsoleToken)
	}

	consoleURL := os.Getenv(controller.EnvConsoleURL)
	if consoleURL == "" {
		return nil, fmt.Errorf("%s environment variable is required", controller.EnvConsoleURL)
	}

	consoleURL = strings.TrimSuffix(consoleURL, "/")
	consoleURL = strings.TrimSuffix(consoleURL, "/gql")
	consoleURL = strings.TrimSuffix(consoleURL, "/ext/gql")

	return &PluralCredentials{
		AccessToken: accessToken,
		ConsoleURL:  fmt.Sprintf("%s/gql", consoleURL),
	}, nil
}

// Start starts the MCP server with stdio transport
func (m *MCPServer) Start() error {
	klog.InfoS("Started Plural Console MCP Server", "consoleURL", m.creds.ConsoleURL)
	return server.ServeStdio(m.server)
}

// registerTools registers all available tools with the MCP server
func (m *MCPServer) registerTools() {
	// Define the agentPullRequest tool
	tool := mcp.NewTool("agentPullRequest",
		mcp.WithDescription("Create a pull request through the Plural console GraphQL API for agent-generated changes"),
		mcp.WithString("runId",
			mcp.Required(),
			mcp.Description("The agent run ID associated with this pull request"),
		),
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
	)

	// Add the tool with our handler
	m.server.AddTool(tool, m.agentPullRequestHandler)
}

// agentPullRequestHandler - actual implementation with GraphQL call
func (m *MCPServer) agentPullRequestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Validate required parameters
	err := m.ensureArguments(request, "runId", "repository", "title", "body", "base", "head")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameters: %v", err)), nil
	}

	// Extract required parameters
	runId, _ := request.RequireString("runId")
	repository, _ := request.RequireString("repository")
	title, _ := request.RequireString("title")
	body, _ := request.RequireString("body")
	base, _ := request.RequireString("base")
	head, _ := request.RequireString("head")

	pr, err := m.client.CreateAgentPullRequest(ctx, runId, client.AgentPullRequestAttributes{
		Title:      title,
		Body:       body,
		Repository: repository,
		Base:       base,
		Head:       head,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create pull request: %v", err)), nil
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
		Message:        fmt.Sprintf("Successfully created pull request in %s from %s to %s", repository, head, base),
		PullRequestId:  pr.ID,
		PullRequestUrl: pr.URL,
		Status:         pr.Status,
		Title:          pr.Title,
		Creator:        pr.Creator,
	})
}

func (m *MCPServer) ensureArguments(request mcp.CallToolRequest, args ...string) error {
	for _, arg := range args {
		if _, err := request.RequireString(arg); err != nil {
			return fmt.Errorf("missing %s: %w", arg, err)
		}
	}

	return nil
}
