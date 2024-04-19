package client

import (
	"errors"

	console "github.com/pluralsh/console-client-go"
	"github.com/samber/lo"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

const (
	stackRunMockID = "mock"
)

var (
	errorNotFound = errors.New("no stack run found")
	stackRunMock = &console.StackRunFragment{
		ID:            stackRunMockID,
		Name:          stackRunMockID,
		Type:          console.StackTypeTerraform,
		Status:        console.StackStatusPending,
		Approval:      lo.ToPtr(false),
		ApprovedAt:    nil,
		Tarball:       "",
		State:         nil,
		Approver:      nil,
		Steps:         nil,
		Files:         nil,
		Git:           nil,
		Repository:    nil,
		JobSpec:       nil,
		Configuration: nil,
		Environment:   nil,
		Output:        nil,
		Errors:        nil,
	}
)

func (c *client) GetStackRun(id string) (*console.StackRunFragment, error) {
	if id == stackRunMockID {
		return stackRunMock, nil
	}

	stackRun, err := c.consoleClient.GetStackRun(c.ctx, id)
	if err != nil {
		return nil, err
	}

	if stackRun == nil || stackRun.StackRun == nil {
		klog.ErrorS(errorNotFound, "id", id)
		return nil, errorNotFound
	}

	klog.V(log.LogLevelInfo).InfoS("found stack run", "id", id)
	return stackRun.StackRun, nil
}
