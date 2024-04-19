package environment

import (
	gqlclient "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/cmd/harness/args"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

const (
	defaultWorkingDir = "stackrun"
)

func (in *environment) Prepare() error {
	if _, err := helpers.Fetch().Tarball(
		in.stackRun.Tarball,
		helpers.FetchWithBearer(args.ConsoleToken()),
		helpers.FetchToDir(in.dir),
	); err != nil {
		klog.ErrorS(err, "failed preparing tarball", "path", in.dir)
		return err
	}

	klog.InfoS("successfully downloaded and unpacked tarball", "path", in.dir)
	return nil
}

// defaults ensures that all required values are initialized
func (in *environment) defaults() Environment {
	if len(in.dir) == 0 {
		helpers.EnsureDirOrDie(defaultWorkingDir)
	}

	return in
}

func WithWorkingDir(dir string) Option {
	return func(e *environment) {
		e.dir = dir
	}
}

func New(stackRun *gqlclient.StackRunFragment, options ...Option) Environment {
	if stackRun == nil {
		klog.Fatal("could not initialize environment: stackRun is nil")
		return nil
	}

	result := &environment{
		stackRun: stackRun,
	}

	for _, opt := range options {
		opt(result)
	}

	return result.defaults()
}
