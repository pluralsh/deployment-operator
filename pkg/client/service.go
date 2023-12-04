package client

import (
	console "github.com/pluralsh/console-client-go"
)

func (c *Client) GetServices() ([]*console.ServiceDeploymentBaseFragment, error) {
	resp, err := c.consoleClient.ListClusterServices(c.ctx)
	if err != nil {
		return nil, err
	}

	return resp.ClusterServices, nil
}

func (c *Client) GetService(id string) (*console.ServiceDeploymentExtended, error) {
	resp, err := c.consoleClient.GetServiceDeployment(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return resp.ServiceDeployment, nil
}

func (c *Client) UpdateComponents(id string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.UpdateServiceComponents(c.ctx, id, components, errs)
	return err
}

func (c *Client) AddServiceErrors(id string, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.AddServiceError(c.ctx, id, errs)
	return err
}
