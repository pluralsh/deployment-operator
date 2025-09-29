package environment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"

	types "github.com/pluralsh/deployment-operator/pkg/harness/environment"
)

// Setup implements Environment interface.
func (in *environment) Setup() error {
	if err := in.prepareWorkingDir(); err != nil {
		return fmt.Errorf("failed to prepare working directory: %w", err)
	}

	if err := in.cloneRepository(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	if err := in.setupAICredentials(); err != nil {
		return fmt.Errorf("failed to setup AI credentials: %w", err)
	}

	// TODO set up default MCP server
	// TODO set up plural agent MCP server

	return nil
}

// prepareWorkingDir creates the working directory
func (in *environment) prepareWorkingDir() error {
	if err := os.MkdirAll(in.dir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	klog.V(log.LogLevelInfo).InfoS("working directory prepared", "path", in.dir)
	return nil
}

// cloneRepository clones the target repository using SCM credentials
func (in *environment) cloneRepository() error {
	if in.agentRun.Repository == "" {
		return fmt.Errorf("repository URL is required")
	}

	repoDir := "repository"

	// Build git clone command with credentials
	args := []string{"clone"}

	// Add auth if SCM credentials are available
	if in.agentRun.ScmCreds != nil {
		// Create credentials file
		credFile := filepath.Join(in.dir, ".git-credentials")
		credContent := fmt.Sprintf("https://%s:%s@github.com\n", in.agentRun.ScmCreds.Username, in.agentRun.ScmCreds.Token)
		if err := os.WriteFile(credFile, []byte(credContent), 0600); err != nil {
			return fmt.Errorf("failed to write git credentials: %w", err)
		}

		// Configure git to use the credentials file
		args = append(args, "--config", fmt.Sprintf("credential.helper=store --file=%s", credFile))
	}

	args = append(args, in.agentRun.Repository, repoDir)
	if err := exec.NewExecutable(
		"git",
		exec.WithArgs(args),
		exec.WithDir(in.dir),
	).Run(context.Background()); err != nil {
		return err
	}

	klog.V(log.LogLevelInfo).InfoS("repository cloned", "url", in.agentRun.Repository, "dir", repoDir)
	return nil
}

// setupAICredentials configures AI service credentials and config files based on runtime type
func (in *environment) setupAICredentials() error {
	if in.agentRun.Runtime == nil {
		return fmt.Errorf("agent runtime information is missing")
	}

	configDir := filepath.Join(in.dir, ".config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	switch in.agentRun.Runtime.Type {
	case gqlclient.AgentRuntimeTypeClaude:
		return in.setupClaudeConfig(configDir)
	case gqlclient.AgentRuntimeTypeGemini:
		return in.setupGeminiConfig(configDir)
	case gqlclient.AgentRuntimeTypeOpencode:
		return in.setupOpenCodeConfig(configDir)
	case gqlclient.AgentRuntimeTypeCustom:
		return in.setupCustomConfig(configDir)
	default:
		return fmt.Errorf("unsupported agent runtime type: %v", in.agentRun.Runtime.Type)
	}
}

// setupClaudeConfig creates Claude CLI configuration
func (in *environment) setupClaudeConfig(configDir string) error {
	// Create Claude config file
	claudeConfigPath := filepath.Join(configDir, "claude")
	if err := os.MkdirAll(claudeConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create claude config directory: %w", err)
	}

	// TODO: Write actual Claude CLI config based on credentials
	configFile := filepath.Join(claudeConfigPath, "config.json")
	config := `{
  "api_key": "` + os.Getenv("CLAUDE_API_KEY") + `",
  "model": "claude-3-5-sonnet-20241022"
}`

	if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write claude config: %w", err)
	}

	klog.V(log.LogLevelDebug).InfoS("claude config created", "path", configFile)
	return nil
}

// setupGeminiConfig creates Gemini CLI configuration
func (in *environment) setupGeminiConfig(configDir string) error {
	// Create Gemini config file
	geminiConfigPath := filepath.Join(configDir, "gemini")
	if err := os.MkdirAll(geminiConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create gemini config directory: %w", err)
	}

	// TODO: Write actual Gemini CLI config based on credentials
	configFile := filepath.Join(geminiConfigPath, "config.json")
	config := `{
  "api_key": "` + os.Getenv("GEMINI_API_KEY") + `",
  "model": "gemini-1.5-pro"
}`

	if err := os.WriteFile(configFile, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write gemini config: %w", err)
	}

	klog.V(log.LogLevelDebug).InfoS("gemini config created", "path", configFile)
	return nil
}

// setupOpenCodeConfig creates OpenCode CLI configuration
func (in *environment) setupOpenCodeConfig(configDir string) error {
	// TODO: Implement OpenCode config setup
	klog.V(log.LogLevelDebug).InfoS("opencode config setup", "dir", configDir)
	return nil
}

// setupCustomConfig creates custom agent configuration
func (in *environment) setupCustomConfig(configDir string) error {
	// TODO: Implement custom agent config setup
	klog.V(log.LogLevelDebug).InfoS("custom agent config setup", "dir", configDir)
	return nil
}

// init ensures that all required values are initialized
func (in *environment) init() types.Environment {
	if in.agentRun == nil {
		klog.Fatal("could not initialize environment: agentRun is nil")
	}

	if len(in.dir) != 0 {
		helpers.EnsureDirOrDie(in.dir)
	}

	return in
}

// New creates a new Environment.
func New(options ...Option) types.Environment {
	result := new(environment)

	for _, opt := range options {
		opt(result)
	}

	return result.init()
}
