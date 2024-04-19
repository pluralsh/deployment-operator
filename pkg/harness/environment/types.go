package environment

import (
	gqlclient "github.com/pluralsh/console-client-go"
)

type Environment interface {
	Prepare() error
}

type environment struct {
	// stackRun ...
	// TODO: doc
	stackRun *gqlclient.StackRunFragment
	// dir ...
	// TODO: doc
	dir string
}

type Option func(*environment)
