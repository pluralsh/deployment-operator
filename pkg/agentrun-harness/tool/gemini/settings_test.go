package gemini

import (
	"encoding/json"
	"testing"

	console "github.com/pluralsh/console/go/client"
)

//nolint:gocyclo
func TestSettingsTemplate_GenerateAndVerifyContents(t *testing.T) {
	baseInput := &ConfigTemplateInput{
		Model:         ModelGemini25Pro,
		RepositoryDir: "/repo",
		ConsoleURL:    "https://console.test",
		ConsoleToken:  "token",
		DeployToken:   "deploy-token",
		AgentRunID:    "run-123",
	}

	t.Run("WRITE mode includes excludeTools for plural MCP server", func(t *testing.T) {
		input := *baseInput
		input.AgentRunMode = console.AgentRunModeWrite

		_, content, err := settings(&input)
		if err != nil {
			t.Fatalf("settings() failed: %v", err)
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(content), &out); err != nil {
			t.Fatalf("generated content is not valid JSON: %v", err)
		}

		mcpServers, ok := out["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers missing or not an object")
		}
		plural, ok := mcpServers["plural"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers.plural missing or not an object")
		}

		excludeTools, hasExclude := plural["excludeTools"]
		if !hasExclude {
			t.Fatal("mcpServers.plural.excludeTools missing in WRITE mode")
		}

		sl, ok := excludeTools.([]any)
		if !ok {
			t.Fatalf("excludeTools is not an array: %T", excludeTools)
		}
		var tools []string
		for _, v := range sl {
			if s, ok := v.(string); ok {
				tools = append(tools, s)
			}
		}
		found := false
		for _, name := range tools {
			if name == "updateAgentRunAnalysis" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("excludeTools must contain updateAgentRunAnalysis in WRITE mode, got: %v", tools)
		}
	})

	t.Run("ANALYZE mode sets includeTools to only updateAgentRunAnalysis for plural MCP server", func(t *testing.T) {
		input := *baseInput
		input.AgentRunMode = console.AgentRunModeAnalyze

		_, content, err := settings(&input)
		if err != nil {
			t.Fatalf("settings() failed: %v", err)
		}

		var out map[string]any
		if err := json.Unmarshal([]byte(content), &out); err != nil {
			t.Fatalf("generated content is not valid JSON: %v", err)
		}

		mcpServers, ok := out["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers missing or not an object")
		}
		plural, ok := mcpServers["plural"].(map[string]any)
		if !ok {
			t.Fatal("mcpServers.plural missing or not an object")
		}

		includeTools, hasInclude := plural["includeTools"]
		if !hasInclude {
			t.Fatal("mcpServers.plural.includeTools missing in ANALYZE mode")
		}
		sl, ok := includeTools.([]any)
		if !ok {
			t.Fatalf("includeTools is not an array: %T", includeTools)
		}
		var tools []string
		for _, v := range sl {
			if s, ok := v.(string); ok {
				tools = append(tools, s)
			}
		}
		if len(tools) != 1 || tools[0] != "updateAgentRunAnalysis" {
			t.Errorf("includeTools must be exactly [\"updateAgentRunAnalysis\"] in ANALYZE mode, got: %v", tools)
		}
	})

	t.Run("coreTools differ by mode", func(t *testing.T) {
		writeInput := *baseInput
		writeInput.AgentRunMode = console.AgentRunModeWrite
		_, writeContent, err := settings(&writeInput)
		if err != nil {
			t.Fatalf("settings() WRITE failed: %v", err)
		}

		analyzeInput := *baseInput
		analyzeInput.AgentRunMode = console.AgentRunModeAnalyze
		_, analyzeContent, err := settings(&analyzeInput)
		if err != nil {
			t.Fatalf("settings() ANALYZE failed: %v", err)
		}

		var writeOut, analyzeOut map[string]any
		if err := json.Unmarshal([]byte(writeContent), &writeOut); err != nil {
			t.Fatalf("WRITE content not valid JSON: %v", err)
		}
		if err := json.Unmarshal([]byte(analyzeContent), &analyzeOut); err != nil {
			t.Fatalf("ANALYZE content not valid JSON: %v", err)
		}

		writeCoreTools, _ := writeOut["coreTools"].([]any)
		analyzeCoreTools, _ := analyzeOut["coreTools"].([]any)

		hasWriteFile := false
		for _, t := range writeCoreTools {
			if s, ok := t.(string); ok && s == "WriteFileTool" {
				hasWriteFile = true
				break
			}
		}
		if !hasWriteFile {
			t.Error("WRITE mode coreTools should include WriteFileTool")
		}

		hasWriteInAnalyze := false
		for _, t := range analyzeCoreTools {
			if s, ok := t.(string); ok && (s == "WriteFileTool" || s == "EditTool") {
				hasWriteInAnalyze = true
				break
			}
		}
		if hasWriteInAnalyze {
			t.Error("ANALYZE mode coreTools should not include WriteFileTool or EditTool")
		}
	})
}
