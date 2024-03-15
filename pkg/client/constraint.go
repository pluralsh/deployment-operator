package client

import console "github.com/pluralsh/console-client-go"

func (c *client) UpsertConstraints(constrains []*console.PolicyConstraintAttributes) (*console.UpsertPolicyConstraints, error) {
	return c.consoleClient.UpsertPolicyConstraints(c.ctx, constrains)
}
