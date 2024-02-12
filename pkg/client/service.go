package client

import (
	"fmt"
	console "github.com/pluralsh/console-client-go"
)

func (c *client) GetServices(after *string, first *int64) (*console.PagedClusterServices, error) {

	resp, err := c.consoleClient.PagedClusterServices(c.ctx, after, first, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.GetPagedClusterServices() == nil {
		return nil, fmt.Errorf("the response from PagedClusterServices is nil")
	}
	return resp, nil
}

func (c *client) GetService(id string) (*console.GetServiceDeploymentForAgent_ServiceDeployment, error) {
	resp, err := c.consoleClient.GetServiceDeploymentForAgent(c.ctx, id)
	if err != nil {
		return nil, err
	}

	return resp.ServiceDeployment, nil
}

func (c *client) UpdateComponents(id string, components []*console.ComponentAttributes, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.UpdateServiceComponents(c.ctx, id, components, errs)
	return err
}

func (c *client) AddServiceErrors(id string, errs []*console.ServiceErrorAttributes) error {
	_, err := c.consoleClient.AddServiceError(c.ctx, id, errs)
	return err
}
