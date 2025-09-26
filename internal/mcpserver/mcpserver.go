package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
)

// PluralCredentials holds the Plural console credentials
type PluralCredentials struct {
	AccessToken string
	ConsoleURL  string
}

// MCPServer wraps the mcp server with Plural credentials
type MCPServer struct {
	server *server.MCPServer
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

	// Ensure the console URL has the correct GraphQL endpoint
	if !strings.HasSuffix(consoleURL, "/gql") && !strings.HasSuffix(consoleURL, "/ext/gql") {
		// Add the GraphQL endpoint if not present
		if strings.HasSuffix(consoleURL, "/") {
			consoleURL = consoleURL + "ext/gql"
		} else {
			consoleURL = consoleURL + "/ext/gql"
		}
	}

	return &PluralCredentials{
		AccessToken: accessToken,
		ConsoleURL:  consoleURL,
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
		mcp.WithString("repository",
			mcp.Required(),
			mcp.Description("The repository name (e.g., 'myorg/myrepo')"),
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
	// Extract required parameters
	runId, err := request.RequireString("runId")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing runId: %v", err)), nil
	}

	repository, err := request.RequireString("repository")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing repository: %v", err)), nil
	}

	title, err := request.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing title: %v", err)), nil
	}

	body, err := request.RequireString("body")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing body: %v", err)), nil
	}

	base, err := request.RequireString("base")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing base: %v", err)), nil
	}

	head, err := request.RequireString("head")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing head: %v", err)), nil
	}

	// Execute the GraphQL mutation
	result, err := m.executeAgentPullRequestMutation(ctx, runId, repository, title, body, base, head)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create pull request: %v", err)), nil
	}

	// Return successful result
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// executeAgentPullRequestMutation executes the actual GraphQL mutation
func (m *MCPServer) executeAgentPullRequestMutation(ctx context.Context, runId, repository, title, body, base, head string) (map[string]interface{}, error) {
	// GraphQL mutation based on the Plural console schema
	mutation := `
		mutation AgentPullRequest($runId: ID!, $attributes: AgentPullRequestAttributes!) {
			agentPullRequest(runId: $runId, attributes: $attributes) {
				id
				status
				url
				title
			}
		}
	`

	// Prepare variables according to the schema
	variables := map[string]interface{}{
		"runId": runId,
		"attributes": map[string]interface{}{
			"title":      title,
			"body":       body,
			"repository": repository,
			"base":       base,
			"head":       head,
		},
	}

	// Execute the GraphQL request
	response, err := m.callPluralGraphQL(ctx, mutation, variables)
	if err != nil {
		return nil, fmt.Errorf("GraphQL request failed: %w", err)
	}

	// Extract the agentPullRequest result
	agentPR, ok := response["agentPullRequest"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from agentPullRequest mutation")
	}

	return map[string]interface{}{
		"success":        true,
		"pullRequestId":  getString(agentPR, "id"),
		"pullRequestUrl": getString(agentPR, "url"),
		"status":         getString(agentPR, "status"),
		"title":          getString(agentPR, "title"),
		"message":        fmt.Sprintf("Successfully created pull request in %s from %s to %s", repository, head, base),
	}, nil
}

// callPluralGraphQL executes a GraphQL request against the Plural console API
func (m *MCPServer) callPluralGraphQL(ctx context.Context, mutation string, variables map[string]interface{}) (map[string]interface{}, error) {
	// Prepare the GraphQL request payload
	payload := map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", m.creds.ConsoleURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.creds.AccessToken)

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var result struct {
		Data   map[string]interface{} `json:"data"`
		Errors []struct {
			Message string        `json:"message"`
			Path    []interface{} `json:"path,omitempty"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	// Check for HTTP errors
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}

	return result.Data, nil
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
