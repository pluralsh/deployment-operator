package client

import (
	"errors"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

var (
	errorNotFound = errors.New("no stack run found")
)

func (c *client) GetStackRun(id string) (result *harness.StackRun, err error) {
	stackRun, err := c.consoleClient.GetStackRunBase(c.ctx, id)
	if err != nil {
		return nil, err
	}

	if stackRun == nil || stackRun.StackRun == nil {
		klog.ErrorS(errorNotFound, "id", id)
		return nil, errorNotFound
	}

	klog.V(log.LogLevelInfo).InfoS("found stack run", "id", id)
	return result.FromStackRunBaseFragment(stackRun.StackRun), nil
}
