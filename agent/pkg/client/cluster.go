package client

import (
	console "github.com/pluralsh/console-client-go"
)

func (c *Client) Ping(vsn string) error {
	_, err := c.consoleClient.PingCluster(c.ctx, console.ClusterPing{CurrentVersion: vsn})
	return err
}
