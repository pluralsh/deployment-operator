package controller

import (
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

func WithAgentRun(id string) Option {
	return func(s *agentRunController) {
		s.agentRunID = id
	}
}

func WithConsoleClient(client console.Client) Option {
	return func(s *agentRunController) {
		s.consoleClient = client
	}
}

func WithWorkingDir(dir string) Option {
	return func(s *agentRunController) {
		s.dir = dir
	}
}

func WithSinkOptions(options ...sink.Option) Option {
	return func(s *agentRunController) {
		s.sinkOptions = options
	}
}

func WithExecOptions(options ...exec.Option) Option {
	return func(s *agentRunController) {
		s.execOptions = options
	}
}

func WithConsoleToken(token string) Option {
	return func(s *agentRunController) {
		s.consoleToken = token
	}
}

func WithConsoleUrl(url string) Option {
	return func(s *agentRunController) {
		s.consoleUrl = url
	}
}
