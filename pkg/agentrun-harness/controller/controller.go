package controller

import (
	"context"
	"fmt"
	"path/filepath"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/klog/v2"

	agentrunv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/agentrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/environment"
	"github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool"
	toolv1 "github.com/pluralsh/deployment-operator/pkg/agentrun-harness/tool/v1"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
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

	in.tool.Run(
		ctx,
		exec.WithHook(v1.LifecyclePreStart, in.preExecHook()),
		exec.WithHook(v1.LifecyclePostStart, in.postExecHook()),
	)

	ready = true
	in.Unlock()
	select {
	// Stop the execution if provided context is done.
	case <-ctx.Done():
		retErr = context.Cause(ctx)
	// In case of any error finish the execution and return error.
	case err := <-in.errChan:
		retErr = err
	// If execution finished successfully, return without error.
	case <-in.done:
		retErr = nil
	}

	klog.V(log.LogLevelExtended).InfoS("all subroutines finished")
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

	in.tool = tool.New(in.agentRun.Runtime.Type, toolv1.Config{
		WorkDir:       in.dir,
		RepositoryDir: filepath.Join(in.dir, "repository"),
		FinishedChan:  in.done,
		ErrorChan:     in.errChan,
		Run:           in.agentRun,
	})

	return in.tool.Configure(in.consoleUrl, *in.agentRun.PluralCreds.Token, in.deployToken)
}

// completeAgentRun updates the agent run status in the Console API
func (in *agentRunController) completeAgentRun(status gqlclient.AgentRunStatus, agentRunErr error) error {
	var errorMsg *string
	if agentRunErr != nil {
		msg := agentRunErr.Error()
		errorMsg = &msg
	}

	statusAttrs := gqlclient.AgentRunStatusAttributes{
		Status:   status,
		Error:    errorMsg,
		Messages: in.tool.Messages(),
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
		errChan: errChan,
		done:    finishedChan,
		dir:     "/plural", // default working directory from pod spec
	}

	for _, option := range opts {
		option(ctrl)
	}

	return ctrl.init()
}
