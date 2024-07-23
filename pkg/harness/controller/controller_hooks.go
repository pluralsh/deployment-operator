package controller

import (
	"context"
	"errors"
	"time"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	internalerrors "github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

var (
	runApproved = false
)

// preStart function is executed before stack run steps.
func (in *stackRunController) preStart() {
	if in.stackRun.Status != gqlclient.StackStatusPending && !environment.IsDev() {
		klog.Fatalf("could not start stack run: invalid status: %s", in.stackRun.Status)
	}

	if err := stackrun.StartStackRun(in.consoleClient, in.stackRunID); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}

	if in.stackRun.ManageState {
		err := in.tool.ConfigureStateBackend("harness", in.consoleToken, in.stackRun.StateUrls)
		if err != nil {
			klog.Fatalf("could not configure state backend: %v", err)
		}
	}
}

// postStart function is executed after all stack run steps.
func (in *stackRunController) postStart(err error) {
	var status gqlclient.StackStatus

	switch {
	case err == nil:
		status = gqlclient.StackStatusSuccessful
	case errors.Is(err, internalerrors.ErrRemoteCancel):
		status = gqlclient.StackStatusCancelled
	default:
		status = gqlclient.StackStatusFailed
	}

	if err := in.completeStackRun(status, err); err != nil {
		_ = stackrun.MarkStackRun(in.consoleClient, in.stackRunID, gqlclient.StackStatusFailed)
		klog.ErrorS(err, "could not complete stack run")
	}

	klog.V(log.LogLevelInfo).InfoS("stack run completed")
}

// postStepRun is a callback function started by the executor after executable finishes.
// It provides the information about run step that was executed and if it exited with error
// or not.
func (in *stackRunController) postStepRun(id string, err error) {
	var status gqlclient.StepStatus

	switch {
	case err == nil:
		status = gqlclient.StepStatusSuccessful
	default:
		status = gqlclient.StepStatusFailed
	}

	if err := stackrun.MarkStackRunStep(in.consoleClient, id, status); err != nil {
		klog.ErrorS(err, "could not update stack run step status")
	}
}

// postExecHook is a callback function started by the exec.Executable after it finishes.
// Unlike postStepRun it does not provide any additional information.
func (in *stackRunController) postExecHook(step *gqlclient.RunStepFragment) v1.HookFunction {
	return func() error {
		if step.Stage != gqlclient.StepStagePlan {
			return nil
		}

		return in.uploadPlan()
	}
}

// postExecHook is a callback function started by the exec.Executable before it runs the executable.
func (in *stackRunController) preExecHook(step *gqlclient.RunStepFragment) v1.HookFunction {
	return func() error {
		if (step.Stage == gqlclient.StepStageApply || step.Stage == gqlclient.StepStageDestroy) && in.requiresApproval() {
			in.waitForApproval()
		}

		if err := stackrun.StartStackRunStep(in.consoleClient, step.ID); err != nil {
			klog.ErrorS(err, "could not update stack run status")
		}

		return nil
	}
}

func (in *stackRunController) requiresApproval() bool {
	return in.stackRun.Approval && !runApproved && in.stackRun.ApprovedAt == nil
}

func (in *stackRunController) waitForApproval() {
	// Retry here to make sure that the pending approval status will be set before we start waiting.
	stackrun.MarkStackRunWithRetry(in.consoleClient, in.stackRunID, gqlclient.StackStatusPendingApproval, 5*time.Second)

	klog.V(log.LogLevelInfo).InfoS("waiting for approval to proceed")
	// Condition function never returns error. We can ignore it.
	_ = wait.PollUntilContextCancel(context.Background(), 5*time.Second, true, func(_ context.Context) (done bool, err error) {
		if runApproved {
			return true, nil
		}

		stack, err := in.consoleClient.GetStackRun(in.stackRunID)
		if err != nil {
			klog.ErrorS(err, "could not check stack run approval")
			return false, nil
		}

		runApproved = stack.ApprovedAt != nil
		return runApproved, nil
	})

	// Retry here to make sure that we resume the stack run status to running after it has been approved.
	stackrun.MarkStackRunWithRetry(in.consoleClient, in.stackRunID, gqlclient.StackStatusRunning, 5*time.Second)
}

func (in *stackRunController) uploadPlan() error {
	state, err := in.tool.Plan()
	if err != nil {
		klog.ErrorS(err, "could not prepare plan")
	}

	return in.consoleClient.UpdateStackRun(in.stackRunID, gqlclient.StackRunAttributes{
		State:  state,
		Status: gqlclient.StackStatusRunning,
	})
}
