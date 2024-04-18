package client

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
)

func (c *client) GetStuckRun(id string) (*console.StackRunFragment, error) {
	restore, err := c.consoleClient.GetStackRun(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return restore.StackRun, nil
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
