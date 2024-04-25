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

func WithCustomErrorSink(sink io.Writer) Option {
	return func(e *executable) {
		e.errorLogSink = sink
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
