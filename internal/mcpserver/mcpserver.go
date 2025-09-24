package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

// PluralCredentials holds the Plural console credentials
type PluralCredentials struct {
	AccessToken string
	ConsoleURL  string
}

// AgentPullRequestInput represents the input for the agent pull request mutation
type AgentPullRequestInput struct {
	Repository  string                 `json:"repository"`
	Branch      string                 `json:"branch,omitempty"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Changes     []FileChange           `json:"changes"`
	AgentRunID  string                 `json:"agentRunId,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FileChange represents a file change in the pull request
type FileChange struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Action  string `json:"action"` // "create", "update", "delete"
}

// AgentPullRequestResult represents the result of creating a pull request
type AgentPullRequestResult struct {
	PullRequestID  string `json:"pullRequestId"`
	PullRequestURL string `json:"pullRequestUrl"`
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
}

// MCPServer wraps the mcp server with Plural console client
type MCPServer struct {
	server        *server.MCPServer
	consoleClient console.ConsoleClient
	credentials   *PluralCredentials
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer() (*MCPServer, error) {
	// Load Plural credentials from environment
	creds, err := loadPluralCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load Plural credentials: %w", err)
	}

	// Create console client
	consoleClient := console.NewClient(&http.Client{
		Transport: helpers.NewAuthorizationTokenTransport(creds.AccessToken),
	}, creds.ConsoleURL, nil)

	// Create MCP server with capabilities
	mcpServer := server.NewMCPServer(
		"Plural Console GraphQL MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	mcpServerWrapper := &MCPServer{
		server:        mcpServer,
		consoleClient: consoleClient,
		credentials:   creds,
	}

	// Register the agentPullRequest tool
	if err := mcpServerWrapper.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return mcpServerWrapper, nil
}

// loadPluralCredentials loads Plural credentials from environment variables
func loadPluralCredentials() (*PluralCredentials, error) {
	accessToken := os.Getenv("PLURAL_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, fmt.Errorf("PLURAL_ACCESS_TOKEN environment variable is required")
	}

	consoleURL := os.Getenv("PLURAL_CONSOLE_URL")
	if consoleURL == "" {
		return nil, fmt.Errorf("PLURAL_CONSOLE_URL environment variable is required")
	}

	// Ensure the console URL has the correct GraphQL endpoint
	if !strings.HasSuffix(consoleURL, "/gql") && !strings.HasSuffix(consoleURL, "/ext/gql") {
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

// registerTools registers all available tools with the MCP server
func (s *MCPServer) registerTools() error {
	// Define the input schema
	schema := mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]any{
			"repository": map[string]any{
				"type":        "string",
				"description": "The repository URL or identifier",
			},
			"branch": map[string]any{
				"type":        "string",
				"description": "The branch name for the pull request (optional, defaults to auto-generated)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "The title of the pull request",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "The description of the pull request (optional)",
			},
			"changes": map[string]any{
				"type":        "array",
				"description": "Array of file changes to include in the pull request",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "File path",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "File content",
						},
						"action": map[string]any{
							"type":        "string",
							"description": "Action to perform: create, update, or delete",
							"enum":        []string{"create", "update", "delete"},
						},
					},
					"required": []string{"path", "action"},
				},
			},
			"agentRunId": map[string]any{
				"type":        "string",
				"description": "The ID of the agent run associated with this pull request (optional)",
			},
			"metadata": map[string]any{
				"type":        "object",
				"description": "Additional metadata for the pull request (optional)",
			},
		},
		Required: []string{"repository", "title", "changes"},
	}

	// Register the agentPullRequest tool
	agentPullRequestTool := mcp.NewTool(
		"agentPullRequest",
		mcp.WithDescription("Create a pull request through the Plural console GraphQL API for agent-generated changes"),
	)
	agentPullRequestTool.InputSchema = schema

	s.server.AddTool(agentPullRequestTool, s.handleAgentPullRequest)
	return nil
}

// handleAgentPullRequest handles the agentPullRequest tool call
func (s *MCPServer) handleAgentPullRequest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse the input arguments
	var input AgentPullRequestInput
	argsBytes, ok := req.Params.Arguments.([]byte)
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}
	if err := json.Unmarshal(argsBytes, &input); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse input arguments: %v", err)), nil
	}

	// Validate required fields
	if input.Repository == "" {
		return mcp.NewToolResultError("Repository is required"), nil
	}
	if input.Title == "" {
		return mcp.NewToolResultError("Title is required"), nil
	}
	if len(input.Changes) == 0 {
		return mcp.NewToolResultError("At least one file change is required"), nil
	}

	// Execute the GraphQL mutation
	result, err := s.executeAgentPullRequestMutation(ctx, input)
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
func (s *MCPServer) executeAgentPullRequestMutation(ctx context.Context, input AgentPullRequestInput) (*AgentPullRequestResult, error) {
	log.Printf("Executing agentPullRequest mutation for repository: %s", input.Repository)
	log.Printf("Title: %s", input.Title)
	log.Printf("Changes: %d files", len(input.Changes))

	// GraphQL mutation based on the Plural console schema
	mutation := `
		mutation AgentPullRequest($input: AgentPullRequestInput!) {
			agentPullRequest(input: $input) {
				id
				url
				status
			}
		}
	`

	// Convert our input to match the expected GraphQL input format
	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"repository":  input.Repository,
			"title":       input.Title,
			"description": input.Description,
			"changes":     input.Changes,
			"agentRunId":  input.AgentRunID,
			"metadata":    input.Metadata,
		},
	}

	// Execute the GraphQL mutation
	// Note: The console client doesn't have a generic Execute method, so we need to
	// implement this using HTTP requests directly or extend the console client
	response, err := s.executeGraphQLMutation(ctx, mutation, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GraphQL mutation: %w", err)
	}

	// Parse the response
	agentPR, ok := response["agentPullRequest"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format from agentPullRequest mutation")
	}

	result := &AgentPullRequestResult{
		PullRequestID:  getString(agentPR, "id"),
		PullRequestURL: getString(agentPR, "url"),
		Status:         getString(agentPR, "status"),
		Message:        fmt.Sprintf("Successfully created pull request for %s with %d file changes", input.Repository, len(input.Changes)),
	}

	return result, nil
}

// executeGraphQLMutation executes a raw GraphQL mutation using HTTP
func (s *MCPServer) executeGraphQLMutation(ctx context.Context, mutation string, variables map[string]interface{}) (map[string]interface{}, error) {
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
	req, err := http.NewRequestWithContext(ctx, "POST", s.credentials.ConsoleURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.credentials.AccessToken)

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
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
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

// Start starts the MCP server with stdio transport
func (s *MCPServer) Start() error {
	log.Println("Starting Plural Console GraphQL MCP Server...")
	log.Printf("Console URL: %s", s.credentials.ConsoleURL)
	log.Println("Using stdio transport for local communication")

	// Start the server with stdio transport (suitable for local pod execution)
	return s.Start()
}
