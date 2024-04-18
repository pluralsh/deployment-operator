package client

import (
	"fmt"

	console "github.com/pluralsh/console-client-go"
)

func (c *client) GetInfrastructureStack(id string) (*console.InfrastructureStackFragment, error) {
	restore, err := c.consoleClient.GetInfrastructureStack(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return restore.InfrastructureStack, nil
}

func (c *client) ListInfrastructureStacks(after *string, first *int64) (*console.ListInfrastructureStacks_InfrastructureStacks, error) {
	resp, err := c.consoleClient.ListInfrastructureStacks(c.ctx, after, first, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.InfrastructureStacks == nil {
		return nil, fmt.Errorf("the response from ListInfrastructureStacks is nil")
	}
	return resp.InfrastructureStacks, nil
}
