package client

import (
	console "github.com/pluralsh/console/go/client"
)

func (c *client) SaveUpgradeInsights(attributes []*console.UpgradeInsightAttributes) (*console.SaveUpgradeInsights, error) {
	return c.consoleClient.SaveUpgradeInsights(c.ctx, attributes)
}
