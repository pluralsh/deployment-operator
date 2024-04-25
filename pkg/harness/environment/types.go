package environment

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness"
)

type Environment interface {
	Setup() error
	WorkingDir() string
}

type environment struct {
	// stackRun ...
	// TODO: doc
	stackRun *harness.StackRun
	// dir ...
	// TODO: doc
	dir string
	// fetchClient
	// TODO: doc
	fetchClient helpers.FetchClient
}

type Option func(*environment)
