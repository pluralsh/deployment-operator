package controller

import (
	"context"
	"errors"
	"time"

	gqlclient "github.com/pluralsh/console-client-go"
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
}

// postStart function is executed after all stack run steps.
func (in *stackRunController) postStart(err error) error {
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
		klog.ErrorS(err, "could not complete stack run")
	}

	klog.V(log.LogLevelInfo).InfoS("stack run completed")
	return err
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
// It differs from the
func (in *stackRunController) postExecHook(stage gqlclient.StepStage) v1.HookFunction {
	return func() error {
		if stage != gqlclient.StepStagePlan {
			return nil
		}

		return in.uploadPlan()
	}
}

func (in *stackRunController) preExecHook(stage gqlclient.StepStage, id string) v1.HookFunction {
	return func() error {
		if stage == gqlclient.StepStageApply {
			if err := in.approvalCheck(); err != nil {
				return err
			}
		}

		if err := stackrun.StartStackRunStep(in.consoleClient, id); err != nil {
			klog.ErrorS(err, "could not update stack run status")
		}

		return nil
	}
}

func (in *stackRunController) approvalCheck() error {
	if !in.stackRun.Approval || runApproved {
		return nil
	}

	if err := stackrun.MarkStackRun(in.consoleClient, in.stackRunID, gqlclient.StackStatusPendingApproval); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}

	klog.V(log.LogLevelInfo).InfoS("waiting for approval to proceed")
	return wait.PollUntilContextCancel(context.Background(), 5*time.Second, true, func(_ context.Context) (done bool, err error) {
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
