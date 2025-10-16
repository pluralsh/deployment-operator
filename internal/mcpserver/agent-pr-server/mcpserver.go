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

	// Cache for the latest agent run data
	cachedAgentRun *client.AgentRunFragment
	cachedRunID    string
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
	agentPullRequestTool := mcp.NewTool("agentPullRequest",
		mcp.WithDescription("Create a pull request through the Plural console GraphQL API for agent-generated changes"),
		mcp.WithString("runId",
			mcp.Required(),
			mcp.Description("The agent run ID associated with this pull request"),
		),
		mcp.WithString("repository",
			mcp.Required(),
			mcp.Description("The repository where the pull request will be created in a format like 'owner/repo'"),
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
	m.server.AddTool(agentPullRequestTool, m.agentPullRequestHandler)

	updateAgentRunAnalysisTool := mcp.NewTool("updateAgentRunAnalysis",
		mcp.WithDescription("Update the analysis of the agent run"),
		mcp.WithString("runId",
			mcp.Required(),
			mcp.Description("The agent run ID"),
		),
		mcp.WithString("summary",
			mcp.Required(),
			mcp.Description("The summary of the analysis"),
		),
		mcp.WithString("analysis",
			mcp.Required(),
			mcp.Description("The analysis of the agent run"),
		),
		mcp.WithArray("bullets",
			mcp.Required(),
			mcp.Description("The bullets of the analysis"),
		),
	)

	m.server.AddTool(updateAgentRunAnalysisTool, m.updateAgentRunAnalysisHandler)

	updateAgentRunTodosTool := mcp.NewTool("updateAgentRunTodos",
		mcp.WithDescription("Update the todos of the agent run"),
		mcp.WithString("runId",
			mcp.Required(),
			mcp.Description("The agent run ID"),
		),
		mcp.WithString("title",
			mcp.Required(),
			mcp.Description("The title of the todo"),
		),
		mcp.WithBoolean("done",
			mcp.Required(),
			mcp.Description("Whether the todo is done"),
		),
	)

	m.server.AddTool(updateAgentRunTodosTool, m.updateAgentRunTodosHandler)

	getAgentRunTodosTool := mcp.NewTool("getAgentRunTodos",
		mcp.WithDescription("Get the current todos list for the agent run"),
		mcp.WithString("runId",
			mcp.Required(),
			mcp.Description("The agent run ID"),
		),
	)

	m.server.AddTool(getAgentRunTodosTool, m.getAgentRunTodosHandler)
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

func (m *MCPServer) updateAgentRunAnalysisHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Validate string parameters
	err := m.ensureArguments(request, "runId", "summary", "analysis")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameters: %v", err)), nil
	}

	// Validate bullets parameter separately with specific error handling
	bullets, err := request.RequireStringSlice("bullets")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid bullets parameter: %v", err)), nil
	}

	// Extract validated parameters
	runId, _ := request.RequireString("runId")
	summary, _ := request.RequireString("summary")
	analysis, _ := request.RequireString("analysis")

	// Convert []string to []*string for the client call
	bulletPointers := make([]*string, len(bullets))
	for i := range bullets {
		bulletPointers[i] = &bullets[i]
	}

	agentRun, err := m.client.UpdateAgentRunAnalysis(ctx, runId, client.AgentAnalysisAttributes{
		Summary:  summary,
		Analysis: analysis,
		Bullets:  bulletPointers,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent run analysis: %v", err)), nil
	}

	// Cache the updated agent run
	m.cachedAgentRun = agentRun
	m.cachedRunID = runId

	return mcp.NewToolResultJSON(struct {
		Success  bool     `json:"success"`
		Message  string   `json:"message"`
		Summary  string   `json:"summary"`
		Analysis string   `json:"analysis"`
		Bullets  []string `json:"bullets"`
	}{
		Success:  true,
		Message:  fmt.Sprintf("Successfully updated agent run analysis for %s", runId),
		Summary:  summary,
		Analysis: analysis,
		Bullets:  bullets,
	})
}

func (m *MCPServer) updateAgentRunTodosHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	err := m.ensureArguments(request, "runId", "title", "done")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameters: %v", err)), nil
	}

	runId, _ := request.RequireString("runId")
	title, _ := request.RequireString("title")
	done, _ := request.RequireBool("done")

	agentRun, err := m.client.UpdateAgentRunTodos(ctx, runId, []*client.AgentTodoAttributes{
		{
			Title: title,
			Done:  done,
		},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent run todos: %v", err)), nil
	}

	// Cache the updated agent run
	m.cachedAgentRun = agentRun
	m.cachedRunID = runId

	return mcp.NewToolResultJSON(struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Title   string `json:"title"`
		Done    bool   `json:"done"`
	}{
		Success: true,
		Message: fmt.Sprintf("Successfully updated agent run todos for %s", runId),
		Title:   title,
		Done:    done,
	})
}

func (m *MCPServer) getAgentRunTodosHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	err := m.ensureArguments(request, "runId")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing required parameters: %v", err)), nil
	}

	runId, _ := request.RequireString("runId")

	// If we don't have cached data for this run ID, fetch it from the API
	if m.cachedAgentRun == nil || m.cachedRunID != runId {
		agentRun, err := m.client.GetAgentRun(ctx, runId)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get agent run: %v", err)), nil
		}
		m.cachedAgentRun = agentRun
		m.cachedRunID = runId
	}

	todos := make([]map[string]interface{}, 0)
	if m.cachedAgentRun.Todos != nil {
		for _, todo := range m.cachedAgentRun.Todos {
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
		Message: fmt.Sprintf("Retrieved todos for agent run %s from cache", runId),
		RunId:   runId,
		Todos:   todos,
		Source:  "cache",
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
