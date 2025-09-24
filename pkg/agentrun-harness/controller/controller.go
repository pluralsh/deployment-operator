package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

// Start starts the manager and waits indefinitely.
// There are a couple of ways to have start return:
//   - an error has occurred in one of the internal operations
//   - all commands have finished their execution
//   - it was running for too long and timed out
//   - remote cancellation signal was received and stopped the execution
func (in *agentRunController) Start(ctx context.Context) (retErr error) {
	in.Lock()

	ready := false
	defer func() {
		// Only unlock if we haven't reached
		// the internal readiness condition.
		if !ready {
			in.Unlock()
		}

		// Make sure to always run postStart before exiting
		in.postStart(retErr)
	}()

	if retErr = in.prepare(); retErr != nil {
		return retErr
	}

	in.preStart()

	// Add executables to executor
	for _, e := range in.executables(ctx) {
		if retErr = in.executor.Add(e); retErr != nil {
			return retErr
		}
	}

	if retErr = in.executor.Start(ctx); retErr != nil {
		return fmt.Errorf("could not start executor: %w", retErr)
	}

	ready = true
	in.Unlock()
	select {
	// Stop the execution if provided context is done.
	case <-ctx.Done():
		retErr = context.Cause(ctx)
	// In case of any error finish the execution and return error.
	case err := <-in.errChan:
		retErr = err
	// If execution finished successfully return without error.
	case <-in.finishedChan:
		retErr = nil
	}

	// notify subroutines that we are done
	close(in.stopChan)

	// wait for all subroutines to finish
	in.wg.Wait()
	klog.V(log.LogLevelVerbose).InfoS("all subroutines finished")

	return retErr
}

// prepare sets up the agent run environment and AI credentials
func (in *agentRunController) prepare() error {
	// Ensure working directory exists
	if err := os.MkdirAll(in.dir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Setup AI credentials and config files
	if err := in.setupAICredentials(); err != nil {
		return fmt.Errorf("failed to setup AI credentials: %w", err)
	}

	// Clone the repository
	if err := in.cloneRepository(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	//TODO set up default MCP server
	//TODO set up plural agent MCP server

	return nil
}

// setupAICredentials configures AI service credentials and config files based on runtime type
func (in *agentRunController) setupAICredentials() error {
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
func (in *agentRunController) setupClaudeConfig(configDir string) error {
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
func (in *agentRunController) setupGeminiConfig(configDir string) error {
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
func (in *agentRunController) setupOpenCodeConfig(configDir string) error {
	// TODO: Implement OpenCode config setup
	klog.V(log.LogLevelDebug).InfoS("opencode config setup", "dir", configDir)
	return nil
}

// setupCustomConfig creates custom agent configuration
func (in *agentRunController) setupCustomConfig(configDir string) error {
	// TODO: Implement custom agent config setup
	klog.V(log.LogLevelDebug).InfoS("custom agent config setup", "dir", configDir)
	return nil
}

// cloneRepository clones the target repository using SCM credentials
func (in *agentRunController) cloneRepository() error {
	if in.agentRun.Repository == "" {
		return fmt.Errorf("repository URL is required")
	}

	repoDir := filepath.Join(in.dir, "repository")

	// Build git clone command with credentials
	args := []string{"clone"}

	// Add auth if SCM credentials are available
	if in.agentRun.ScmCreds != nil {
		// Configure git credentials
		args = append(args, "--config", fmt.Sprintf("credential.helper=store --file=%s/.git-credentials", in.dir))

		// Create credentials file
		credFile := filepath.Join(in.dir, ".git-credentials")
		credContent := fmt.Sprintf("https://%s:%s@github.com", in.agentRun.ScmCreds.Username, in.agentRun.ScmCreds.Token)
		if err := os.WriteFile(credFile, []byte(credContent), 0600); err != nil {
			return fmt.Errorf("failed to write git credentials: %w", err)
		}
	}

	args = append(args, in.agentRun.Repository, repoDir)
	if err := exec.NewExecutable(
		"git",
		exec.WithArgs(args),
		exec.WithDir(in.dir),
	).Run(context.Background()); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	klog.V(log.LogLevelInfo).InfoS("repository cloned", "url", in.agentRun.Repository, "dir", repoDir)
	return nil
}

// executables returns the list of executables for this agent run - runs the appropriate CLI
func (in *agentRunController) executables(ctx context.Context) []exec.Executable {
	var executables []exec.Executable

	// Run the appropriate coding agent CLI based on runtime type
	// TODO for each cli agent executable builder, start mcp server as background process
	switch in.agentRun.Runtime.Type {
	case gqlclient.AgentRuntimeTypeClaude:
		executables = append(executables, in.claudeExecutable(ctx))
	case gqlclient.AgentRuntimeTypeGemini:
		executables = append(executables, in.geminiExecutable(ctx))
	case gqlclient.AgentRuntimeTypeOpencode:
		executables = append(executables, in.openCodeExecutable(ctx))
	case gqlclient.AgentRuntimeTypeCustom:
		executables = append(executables, in.customExecutable(ctx))
	default:
		klog.ErrorS(fmt.Errorf("unknown agent runtime type: %v", in.agentRun.Runtime.Type), "unsupported runtime type")
	}

	return executables
}

// claudeExecutable creates an executable for Claude CLI
func (in *agentRunController) claudeExecutable(ctx context.Context) exec.Executable {
	args := []string{
		"--config", filepath.Join(in.dir, ".config", "claude", "config.json"),
		"--prompt", in.agentRun.Prompt,
		"--repository", filepath.Join(in.dir, "repository"),
	}

	// Add mode-specific flags
	if in.agentRun.IsAnalyzeMode() {
		args = append(args, "--analyze-only")
	} else if in.agentRun.IsWriteMode() {
		args = append(args, "--write-code")
	}

	// Add system prompt overrides if needed
	if systemPrompt := in.getSystemPromptOverride(); systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	return in.toExecutable(ctx, "claude-cli", "claude", args)
}

// geminiExecutable creates an executable for Gemini CLI
func (in *agentRunController) geminiExecutable(ctx context.Context) exec.Executable {
	args := []string{
		"--config", filepath.Join(in.dir, ".config", "gemini", "config.json"),
		"--prompt", in.agentRun.Prompt,
		"--repository", filepath.Join(in.dir, "repository"),
	}

	// Add mode-specific flags
	if in.agentRun.IsAnalyzeMode() {
		args = append(args, "--mode", "analyze")
	} else if in.agentRun.IsWriteMode() {
		args = append(args, "--mode", "write")
	}

	// Add system prompt overrides if needed
	if systemPrompt := in.getSystemPromptOverride(); systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	return in.toExecutable(ctx, "gemini-cli", "gemini", args)
}

// openCodeExecutable creates an executable for OpenCode CLI
func (in *agentRunController) openCodeExecutable(ctx context.Context) exec.Executable {
	args := []string{
		"--config", filepath.Join(in.dir, ".config", "opencode", "config.json"),
		"--prompt", in.agentRun.Prompt,
		"--repository", filepath.Join(in.dir, "repository"),
	}

	return in.toExecutable(ctx, "opencode-cli", "opencode", args)
}

// customExecutable creates an executable for custom agent
func (in *agentRunController) customExecutable(ctx context.Context) exec.Executable {
	// TODO: Implement custom agent CLI execution
	args := []string{
		"--prompt", in.agentRun.Prompt,
		"--repository", filepath.Join(in.dir, "repository"),
	}

	return in.toExecutable(ctx, "custom-agent", "custom-agent", args)
}

// getSystemPromptOverride returns system prompt override if configured
func (in *agentRunController) getSystemPromptOverride() string {
	// TODO: Check for system prompt overrides from environment or config
	if override := os.Getenv("AGENT_SYSTEM_PROMPT_OVERRIDE"); override != "" {
		return override
	}
	return ""
}

// toExecutable converts a command into an executable
func (in *agentRunController) toExecutable(ctx context.Context, id, cmd string, args []string) exec.Executable {
	// synchronize executable and underlying console writer with
	// the controller to ensure that it does not exit before
	// ensuring they have completed all work.
	in.wg.Add(1)

	consoleWriter := sink.NewConsoleWriter(
		ctx,
		in.consoleClient,
		append(
			in.sinkOptions,
			sink.WithID(id),
			sink.WithOnFinish(func() {
				// Notify controller that all remaining work
				// has been completed.
				in.wg.Done()
			}),
			sink.WithStopChan(in.stopChan),
		)...,
	)

	// base executable options
	options := in.execOptions
	options = append(
		options,
		exec.WithDir(filepath.Join(in.dir, "repository")), // Run CLI in repository directory
		exec.WithEnv(in.agentRun.Env()),
		exec.WithArgs(args),
		exec.WithID(id),
		exec.WithOutputSinks(consoleWriter),
		exec.WithHook(v1.LifecyclePreStart, in.preExecHook(id)),
		exec.WithHook(v1.LifecyclePostStart, in.postExecHook(id)),
	)

	return exec.NewExecutable(cmd, options...)
}

// completeAgentRun updates the agent run status in the Console API
func (in *agentRunController) completeAgentRun(status gqlclient.AgentRunStatus, agentRunErr error) error {
	var errorMsg *string
	if agentRunErr != nil {
		msg := agentRunErr.Error()
		errorMsg = &msg
	}

	statusAttrs := gqlclient.AgentRunStatusAttributes{
		Status: status,
		Error:  errorMsg,
	}

	_, err := in.consoleClient.UpdateAgentRun(context.Background(), in.agentRunID, statusAttrs)
	return err
}

// init initializes the controller with the agent run data from Console API
func (in *agentRunController) init() (Controller, error) {
	if len(in.agentRunID) == 0 {
		return nil, fmt.Errorf("could not initialize controller: agent run id is empty")
	}

	if in.consoleClient == nil {
		return nil, fmt.Errorf("could not initialize controller: consoleClient is nil")
	}

	// Fetch agent run from Console API
	agentRunFragment, err := in.consoleClient.GetAgentRun(context.Background(), in.agentRunID)
	if err != nil {
		return nil, fmt.Errorf("could not fetch agent run: %w", err)
	}

	// Convert console fragment to harness type
	in.agentRun = (&agentrunv1.AgentRun{}).FromAgentRunFragment(agentRunFragment)

	klog.V(log.LogLevelInfo).InfoS("found agent run",
		"id", in.agentRun.ID,
		"status", in.agentRun.Status,
		"mode", in.agentRun.Mode,
		"type", in.agentRun.Runtime.Type,
		"repository", in.agentRun.Repository)

	return in, nil
}

// NewAgentRunController creates a new agent run controller
func NewAgentRunController(opts ...Option) (Controller, error) {
	finishedChan := make(chan struct{})
	errChan := make(chan error, 1)
	ctrl := &agentRunController{
		errChan:      errChan,
		finishedChan: finishedChan,
		stopChan:     make(chan struct{}),
		wg:           sync.WaitGroup{},
		sinkOptions:  make([]sink.Option, 0),
		dir:          "/plural", // default working directory from pod spec
	}

	ctrl.executor = exec.NewExecutor(
		errChan,
		finishedChan,
		exec.WithPostRunFunc(ctrl.postStepRun),
	)

	for _, option := range opts {
		option(ctrl)
	}

	return ctrl.init()
}
