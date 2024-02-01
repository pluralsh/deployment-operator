package client

import (
	console "github.com/pluralsh/console-client-go"
)

func (c *client) PingCluster(attributes console.ClusterPing) error {
	_, err := c.consoleClient.PingCluster(c.ctx, attributes)
	return err
}

func (c *client) Ping(vsn string) error {
	_, err := c.consoleClient.PingCluster(c.ctx, console.ClusterPing{CurrentVersion: vsn})
	return err
}

func (c *client) RegisterRuntimeServices(svcs map[string]string, serviceId *string) error {
	inputs := make([]*console.RuntimeServiceAttributes, 0)
	for name, version := range svcs {
		inputs = append(inputs, &console.RuntimeServiceAttributes{
			Name:    name,
			Version: version,
		})
	}
	_, err := c.consoleClient.RegisterRuntimeServices(c.ctx, inputs, serviceId)
	return err
}

func (c *client) MyCluster() (*console.MyCluster, error) {
	return c.consoleClient.MyCluster(c.ctx)
}
