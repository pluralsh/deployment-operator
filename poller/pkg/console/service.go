package console

import (
	console "github.com/pluralsh/console-client-go"
)

func (c *Client) GetServices() ([]*console.ServiceDeploymentFragment, error) {
	resp, err := c.client.ListClusterServices(c.ctx)
	if err != nil {
		return nil, err
	}

	return resp.ClusterServices, nil
}
