package exec

import (
	"io"

	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun/v1"
)

func WithDir(workingDirectory string) Option {
	return func(t *executable) {
		t.workingDirectory = workingDirectory
	}
}

func WithLogSink(sink io.WriteCloser) Option {
	return func(e *executable) {
		e.logSink = sink
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

func WithID(id string) Option {
	return func(e *executable) {
		e.id = id
	}
}

func WithHook(lifecycle v1.Lifecycle, fn v1.HookFunction) Option {
	return func(e *executable) {
		e.hookFunctions[lifecycle] = fn
	}
}
