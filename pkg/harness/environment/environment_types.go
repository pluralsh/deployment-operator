package environment

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
)

// Environment is responsible for handling harness working directory.
// It can initialize, download and create files required by the gqlclient.StackRun.
type Environment interface {
	// Setup ensures that the environment is correctly initialized
	// in order to start gqlclient.StackRun.
	//
	// 1. Creates a working dir if it doesn't exist.
	// 2. Downloads the tarball related to stack run and unpacks it into the working dir.
	// 3. Creates any additional files that are part of the gqlclient.StackRun.
	Setup() error
}

type environment struct {
	// stackRun provides all information required to prepare
	// the environment and working directory for the actual
	// execution of the stack run. For example, it provides
	// URL of the tarball with mandatory files needed to run
	// stack run step commands.
	stackRun *stackrun.StackRun
	// dir is a working directory where all files/directories
	// are being created.
	dir string
	// fetchClient is a helper client used to download and unpack the tarball.
	fetchClient helpers.FetchClient
}

// Option allows to modify Environment behavior.
type Option func(*environment)