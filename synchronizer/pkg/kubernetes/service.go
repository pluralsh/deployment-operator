package kubernetes

import (
	platform "github.com/pluralsh/deployment-operator/api/apis/platform/v1alpha1"
)

func (c *Client) GetServices() (platform.DeploymentList, error) {
	services := platform.DeploymentList{}
	err := c.client.List(c.ctx, &platform.DeploymentList{})
	return services, err
}

func (c *Client) CreateService(service platform.Deployment) error {
	return c.client.Create(c.ctx, &service)
}

func (c *Client) UpdateService(service platform.Deployment) error {
	return c.client.Update(c.ctx, &service)
}

func (c *Client) DeleteService(service platform.Deployment) error {
	return c.client.Delete(c.ctx, &service)
}
