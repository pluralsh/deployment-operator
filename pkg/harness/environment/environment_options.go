package environment

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
)

// WithWorkingDir allows changing the default working directory of the Environment.
func WithWorkingDir(dir string) Option {
	return func(e *environment) {
		e.dir = dir
	}
}

// WithFetchClient allows configuring helpers.FetchClient used by the Environment
// to download files.
func WithFetchClient(client helpers.FetchClient) Option {
	return func(e *environment) {
		e.fetchClient = client
	}
}

// WithStackRun provides information about stack run used to initialize
// the Environment.
func WithStackRun(stackRun *stackrun.StackRun) Option {
	return func(e *environment) {
		e.stackRun = stackRun
	}
}
