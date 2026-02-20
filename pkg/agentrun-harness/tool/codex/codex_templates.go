package codex

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

func loadPrompt(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func BuildCodexConfig(agents []AgentInput, mcps []MCPInput) (*CodexConfig, error) {
	cfg := &CodexConfig{
		Profiles:   make(map[string]*Profile),
		MCPServers: make(map[string]*MCPServer),
	}

	// Add profiles
	for _, a := range agents {
		prompt, err := loadPrompt(a.PromptFile)
		if err != nil {
			return nil, err
		}

		cfg.Profiles[a.Name] = &Profile{
			Model:                a.Model,
			SandboxMode:          a.SandboxMode,
			ApprovalPolicy:       a.ApprovalPolicy,
			ModelReasoningEffort: a.ModelReasoningEffort,
			ShellEnvironmentPolicy: &ShellEnvPolicy{
				IncludeOnly: a.AllowedEnvVars,
			},
			Features: &Features{
				WebSearchRequest: a.EnableWebSearch,
				ShellSnapshot:    a.EnableShellCache,
			},
			Prompt:        prompt,
			DisabledTools: a.DisabledTools,
			EnabledTools:  a.EnabledTools,
		}
	}

	// Add MCP servers
	for _, m := range mcps {
		cfg.MCPServers[m.Name] = &MCPServer{
			URL:     m.URL,
			Command: m.Command,
			Args:    m.Args,
			Env:     m.Env,
		}
	}

	return cfg, nil
}

func WriteCodexConfig(basePath string, cfg *CodexConfig) (string, error) {

	filePath := filepath.Join(basePath, "config.toml")
	data, err := toml.Marshal(cfg)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", err
	}

	return filePath, nil
}
