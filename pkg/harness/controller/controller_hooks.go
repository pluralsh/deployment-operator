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
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *stackRunController) preStart() {
	if in.stackRun.Status != gqlclient.StackStatusPending && !environment.IsDev() {
		klog.Fatalf("could not start stack run: invalid status: %s", in.stackRun.Status)
	}

	if err := in.markStackRun(gqlclient.StackStatusRunning); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}
}

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

func (in *stackRunController) postStepRun(id string, err error) {
	var status gqlclient.StepStatus

	switch {
	case err == nil:
		status = gqlclient.StepStatusSuccessful
	default:
		status = gqlclient.StepStatusFailed
	}

	if err := in.markStackRunStep(id, status); err != nil {
		klog.ErrorS(err, "could not update stack run step status")
	}
}

func (in *stackRunController) preStepRun(id string) {
	if err := in.markStackRunStep(id, gqlclient.StepStatusRunning); err != nil {
		klog.ErrorS(err, "could not update stack run status")
	}
}

func (in *stackRunController) postExecHook(stage gqlclient.StepStage, id string) stackrun.HookFunction {
	return func() error {
		if stage != gqlclient.StepStagePlan {
			return nil
		}

		return in.uploadPlan()
	}
}

func (in *stackRunController) preExecHook(stage gqlclient.StepStage, id string) stackrun.HookFunction {
	return func() error {
		if err := in.markStackRunStep(id, gqlclient.StepStatusRunning); err != nil {
			klog.ErrorS(err, "could not update stack run status")
		}

		if stage != gqlclient.StepStageApply {
			return nil
		}

		return in.approvalCheck()
	}
}

func (in *stackRunController) approvalCheck() error {
	if !in.stackRun.Approval {
		return nil
	}

	return wait.PollUntilContextCancel(context.Background(), 5*time.Second, true, func(_ context.Context) (done bool, err error) {
		stack, err := in.consoleClient.GetStackRun(in.stackRunID)
		if err != nil {
			klog.ErrorS(err, "could not check stack run approval")
			return false, nil
		}

		return stack.ApprovedAt != nil, nil
	})
}

func (in *stackRunController) uploadPlan() error {
	state, err := in.tool.Plan()
	if err != nil {
		klog.ErrorS(err, "could not prepare plan")
	}

	return in.consoleClient.UpdateStackRun(in.stackRunID, gqlclient.StackRunAttributes{
		State:  state,
		Status: gqlclient.StackStatusSuccessful,
	})
}
