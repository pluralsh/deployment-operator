package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *executable) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, in.command, in.arguments()...)

	// Configure additional writers so that we can simultaneously write output
	// to multiple destinations
	cmd.Stdout = in.stdout()
	cmd.Stderr = in.stderr()

	// Configure environment of the executable.
	// Root process environment is used as a base and passed in env vars
	// are added on top of that. In case of duplicate keys, custom env
	// vars passed to the executable will override root process env vars.
	cmd.Env = append(os.Environ(), in.env...)

	if len(in.workingDirectory) > 0 {
		cmd.Dir = in.workingDirectory
	}

	klog.V(log.LogLevelVerbose).InfoS("executing", "command", in.Command())
	return cmd.Run()
}

func (in *executable) RunWithOutput(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, in.command, in.arguments()...)

	// Configure additional writers so that we can simultaneously write output
	// to multiple destinations
	cmd.Stderr = os.Stdout

	// Configure environment of the executable.
	// Root process environment is used as a base and passed in env vars
	// are added on top of that. In case of duplicate keys, custom env
	// vars passed to the executable will override root process env vars.
	cmd.Env = append(os.Environ(), in.env...)

	if len(in.workingDirectory) > 0 {
		cmd.Dir = in.workingDirectory
	}

	klog.V(log.LogLevelVerbose).InfoS("executing", "command", in.Command())
	return cmd.Output()
}

func (in *executable) Command() string {
	return fmt.Sprintf("%s %s", in.command, strings.Join(in.arguments(), " "))
}

func (in *executable) ID() string {
	if len(in.id) == 0 {
		in.id = string(uuid.NewUUID())
	}

	return in.id
}

func (in *executable) arguments() []string {
	if in.argsModifier != nil {
		return in.argsModifier(in.args)
	}

	return in.args
}

func (in *executable) stderr() io.Writer {
	if in.errorLogSink != nil {
		return io.MultiWriter(os.Stderr, in.errorLogSink)
	}

	return os.Stderr
}

func (in *executable) stdout() io.Writer {
	if in.standardLogSink != nil {
		return io.MultiWriter(os.Stdout, in.standardLogSink)
	}

	return os.Stdout
}

func NewExecutable(command string, options ...Option) Executable {
	result := &executable{
		command: command,
		args:    make([]string, 0),
		env:     make([]string, 0),
	}

	for _, o := range options {
		o(result)
	}

	return result
}
