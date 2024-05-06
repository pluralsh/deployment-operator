package client

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
)

func (c *client) UpdateStackRunStep(stepID string, attr console.RunStepAttributes) (*console.RunStepFragment, error) {
	update, err := c.consoleClient.UpdateStackRunStep(c.ctx, stepID, attr)
	if err != nil {
		return nil, err
	}

	return update.UpdateRunStep, nil
}

func (c *client) GetStackRun(id string) (*console.StackRunFragment, error) {
	restore, err := c.consoleClient.GetStackRun(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return restore.StackRun, nil
}

func (c *client) UpdateStackRun(id string, attr console.StackRunAttributes) (*console.StackRunBaseFragment, error) {
	restore, err := c.consoleClient.UpdateStackRun(c.ctx, id, attr)
	if err != nil {
		return nil, err
	}

	return restore.UpdateStackRun, nil
}

func (c *client) ListClusterStackRuns(after *string, first *int64) (*console.ListClusterStacks_ClusterStackRuns, error) {
	resp, err := c.consoleClient.ListClusterStacks(c.ctx, after, first, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.ClusterStackRuns == nil {
		return nil, fmt.Errorf("the response from ListInfrastructureStacks is nil")
	}
	return resp.ClusterStackRuns, nil
}
