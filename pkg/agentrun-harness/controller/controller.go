package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment"
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

	klog.V(log.LogLevelVerbose).InfoS("all subroutines finished")

	return retErr
}

// prepare sets up the agent run environment and AI credentials
func (in *agentRunController) prepare() error {
	env := environment.New(
		environment.WithAgentRun(in.agentRun),
		environment.WithWorkingDir(in.dir),
	)

	if err := env.Setup(); err != nil {
		return err
	}

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

	// Add system prompt overrides if needed
	if systemPrompt := in.getSystemPromptOverride(); systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
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
	if override := os.Getenv("AGENT_SYSTEM_PROMPT_OVERRIDE"); override != "" {
		return override
	}
	return ""
}

// toExecutable converts a command into an executable
func (in *agentRunController) toExecutable(_ context.Context, id, cmd string, args []string) exec.Executable {
	// base executable options
	options := in.execOptions
	options = append(
		options,
		exec.WithDir(filepath.Join(in.dir, "repository")), // Run CLI in repository directory
		exec.WithEnv(in.agentRun.Env()),
		exec.WithArgs(args),
		exec.WithID(id),
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
