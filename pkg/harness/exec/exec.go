package exec

import (
	"context"
	"io"
	"os"
	"os/exec"
)

func (in *Executable) Run(args ...string) (err error) {
	cmd := exec.CommandContext(in.context, string(in.command), args...)

	// Configure additional writers so that we can simultaneously write output
	// to multiple destinations
	cmd.Stderr = io.MultiWriter(os.Stderr, in.errorLogSink)
	cmd.Stdout = io.MultiWriter(os.Stdout, in.standardLogSink)

	if len(in.workingDirectory) > 0 {
		cmd.Dir = in.workingDirectory
	}

	return cmd.Run()
}

func (in *Executable) defaults() *Executable {
	if in.context == nil {
		in.context = context.Background()
	}

	return in
}

func WithDir(workingDirectory string) ExecutableOption {
	return func(t *Executable) {
		t.workingDirectory = workingDirectory
	}
}

func WithCancelableContext(ctx context.Context, signals ...Signal) ExecutableOption {
	return func(t *Executable) {
		t.context = NewCancelableContext(ctx, signals...)
	}
}

func WithCustomOutputSink(sink io.Writer) ExecutableOption {
	return func(e *Executable) {
		e.standardLogSink = sink
	}
}

func WithCustomErrorSink(sink io.Writer) ExecutableOption {
	return func(e *Executable) {
		e.errorLogSink = sink
	}
}

func NewExecutable(command Command, options ...ExecutableOption) *Executable {
	executable := &Executable{command: command}

	for _, o := range options {
		o(executable)
	}

	return executable.defaults()
}
