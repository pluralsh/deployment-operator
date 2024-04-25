package client

import (
	gqlclient "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2"

	internalerrors "github.com/pluralsh/deployment-operator/internal/errors"
	"github.com/pluralsh/deployment-operator/pkg/harness"
	"github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (c *client) GetStackRun(id string) (result *harness.StackRun, err error) {
	stackRun, err := c.consoleClient.GetStackRunBase(c.ctx, id)
	if err != nil && !internalerrors.IsNotFound(err) {
		return nil, err
	}

	if stackRun == nil || stackRun.StackRun == nil {
		return nil, errors.ErrNotFound
	}

	klog.V(log.LogLevelInfo).InfoS("found stack run", "id", id, "status", stackRun.StackRun.Status, "type", stackRun.StackRun.Type)
	return result.FromStackRunBaseFragment(stackRun.StackRun), nil
}

func (c *client) AddStackRunLogs(id, logs string) error {
	if _, err := c.consoleClient.AddStackRunLogs(c.ctx, id, gqlclient.RunLogAttributes{
		Logs: logs,
	}); err != nil {
		return err
	}

	klog.V(log.LogLevelExtended).InfoS("updated logs", "id", id)
	return nil
}

func (c *client) CompleteStackRun(id string, attributes gqlclient.StackRunAttributes) error {
	if _, err := c.consoleClient.CompletesStackRun(c.ctx, id, attributes); err != nil {
		return err
	}

	klog.V(log.LogLevelInfo).InfoS("completed stack run", "id", id, "attributes", attributes)
	return nil
}

func (c *client) UpdateStackRun(id string, attributes gqlclient.StackRunAttributes) error {
	if _, err := c.consoleClient.UpdateStackRun(c.ctx, id, attributes); err != nil {
		return err
	}

	klog.V(log.LogLevelExtended).InfoS("updated stack run", "id", id, "attributes", attributes)
	return nil
}

func (c *client) UpdateStackRunStep(id string, attributes gqlclient.RunStepAttributes) error {
	if _, err := c.consoleClient.UpdateStackRunStep(c.ctx, id, attributes); err != nil {
		return err
	}

	klog.V(log.LogLevelExtended).InfoS("updated stack run step", "id", id, "attributes", attributes)
	return nil
}
