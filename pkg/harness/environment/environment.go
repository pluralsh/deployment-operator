package environment

import (
	"path"

	console "github.com/pluralsh/console-client-go"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/helpers"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *environment) Setup() error {
	if err := in.prepareTarball(); err != nil {
		return err
	}

	if err := in.prepareFiles(); err != nil {
		return err
	}

	return nil
}

func (in *environment) State() (*console.StackStateAttributes, error) {
	return in.runner.State()
}

func (in *environment) Output() ([]*console.StackOutputAttributes, error) {
	return in.runner.Output()
}

// Args ...
// TODO: can we find a better place for this?
func (in *environment) Args(stage console.StepStage) exec.ArgsModifier {
	return in.runner.Args(stage)
}

func (in *environment) prepareTarball() error {
	if _, err := in.fetchClient.Tarball(in.stackRun.Tarball); err != nil {
		klog.ErrorS(err, "failed preparing tarball", "path", in.dir)
		return err
	}

	klog.V(log.LogLevelInfo).InfoS("successfully downloaded and unpacked tarball", "path", in.dir)
	return nil
}

func (in *environment) prepareFiles() error {
	if in.stackRun.Files == nil {
		return nil
	}

	for _, fragment := range in.stackRun.Files {
		destination := path.Join(in.dir, fragment.Path)
		if err := helpers.File().Create(destination, fragment.Content); err != nil {
			klog.ErrorS(err, "failed preparing files", "path", destination)
			return err
		}

		klog.V(log.LogLevelInfo).InfoS("successfully created file", "path", destination)
	}

	return nil
}

// init ensures that all required values are initialized
func (in *environment) init() Environment {
	if in.stackRun == nil {
		klog.Fatal("could not initialize environment: stackRun is nil")
	}

	if len(in.dir) != 0 {
		helpers.EnsureDirOrDie(in.dir)
	}

	in.runner = newRunner(in.stackRun.Type, in.dir)

	return in
}

func New(options ...Option) Environment {
	result := new(environment)

	for _, opt := range options {
		opt(result)
	}

	return result.init()
}
