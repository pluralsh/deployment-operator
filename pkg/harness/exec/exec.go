package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/pluralsh/polly/algorithms"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/signals"
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *executable) Run(ctx context.Context) error {
	if err := in.runLifecycleFunction(v1.LifecyclePreStart); err != nil {
		return err
	}

	ctx = signals.NewCancelableContext(ctx, signals.NewTimeoutSignal(in.timeout))
	cmd := exec.CommandContext(ctx, in.command, in.args...)
	w := in.writer()
	defer in.close(in.outputSinks)

	// Configure additional writers so that we can simultaneously write output
	// to multiple destinations
	// Note: We need to use the same writer for stdout and stderr to guarantee
	// 		 thread-safe writing, otherwise output from stdout and stderr could be
	//		 written concurrently and get reordered.
	cmd.Stdout = w
	cmd.Stderr = w

	// Configure environment of the executable.
	// Root process environment is used as a base and passed in env vars
	// are added on top of that. In case of duplicate keys, custom env
	// vars passed to the executable will override root process env vars.
	cmd.Env = append(os.Environ(), in.env...)

	if len(in.workingDirectory) > 0 {
		cmd.Dir = in.workingDirectory
	}

	klog.V(log.LogLevelExtended).InfoS("executing", "command", in.Command())
	if err := cmd.Run(); err != nil {
		if err = context.Cause(ctx); err != nil {
			return err
		}

		return err
	}

	return in.runLifecycleFunction(v1.LifecyclePostStart)
}

func (in *executable) RunWithOutput(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, in.command, in.args...)

	// Configure environment of the executable.
	// Root process environment is used as a base and passed in env vars
	// are added on top of that. In case of duplicate keys, custom env
	// vars passed to the executable will override root process env vars.
	cmd.Env = append(os.Environ(), in.env...)

	if len(in.workingDirectory) > 0 {
		cmd.Dir = in.workingDirectory
	}

	klog.V(log.LogLevelExtended).InfoS("executing", "command", in.Command())
	return cmd.CombinedOutput()
}

func (in *executable) Command() string {
	return fmt.Sprintf("%s %s", in.command, strings.Join(in.args, " "))
}

func (in *executable) ID() string {
	if len(in.id) == 0 {
		in.id = string(uuid.NewUUID())
	}

	return in.id
}

func (in *executable) writer() io.Writer {
	if len(in.outputSinks) > 0 {
		return io.MultiWriter(
			append(
				algorithms.Map(in.outputSinks, func(writer io.WriteCloser) io.Writer {
					return writer
				}),
				os.Stdout)...,
		)
	}
	return os.Stdout
}

func (in *executable) close(writers []io.WriteCloser) {
	if len(writers) <= 0 {
		return
	}

	for _, w := range writers {
		if err := w.Close(); err != nil {
			klog.ErrorS(err, "failed to close writer")
		}
	}
}

func (in *executable) runLifecycleFunction(lifecycle v1.Lifecycle) error {
	if fn, exists := in.hookFunctions[lifecycle]; exists {
		return fn()
	}

	return nil
}

func NewExecutable(command string, options ...Option) Executable {
	result := &executable{
		command:       command,
		args:          make([]string, 0),
		env:           make([]string, 0),
		hookFunctions: make(map[v1.Lifecycle]v1.HookFunction),
	}

	for _, o := range options {
		o(result)
	}

	return result
}
