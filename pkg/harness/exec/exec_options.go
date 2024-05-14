package exec

import (
	"io"
)

func WithDir(workingDirectory string) Option {
	return func(t *executable) {
		t.workingDirectory = workingDirectory
	}
}

func WithCustomOutputSink(sink io.Writer) Option {
	return func(e *executable) {
		e.standardLogSink = sink
	}
}

func WithEnv(env []string) Option {
	return func(e *executable) {
		e.env = env
	}
}

func WithArgs(args []string) Option {
	return func(e *executable) {
		e.args = args
	}
}

func WithArgsModifier(f ArgsModifier) Option {
	return func(e *executable) {
		e.argsModifier = f
	}
}

func WithID(id string) Option {
	return func(e *executable) {
		e.id = id
	}
}
