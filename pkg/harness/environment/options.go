package environment

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness"
)

func WithWorkingDir(dir string) Option {
	return func(e *environment) {
		e.dir = dir
	}
}

func WithFetchClient(client helpers.FetchClient) Option {
	return func(e *environment) {
		e.fetchClient = client
	}
}

func WithStackRun(stackRun *harness.StackRun) Option {
	return func(e *environment) {
		e.stackRun = stackRun
	}
}
