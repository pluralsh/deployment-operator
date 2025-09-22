package controller

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

func WithAgentRun(id string) AgentRunOption {
	return func(s *agentRunController) {
		s.agentRunID = id
	}
}

func WithAgentRunConsoleClient(client console.Client) AgentRunOption {
	return func(s *agentRunController) {
		s.consoleClient = client
	}
}

func WithAgentRunFetchClient(client helpers.FetchClient) AgentRunOption {
	return func(s *agentRunController) {
		s.fetchClient = client
	}
}

func WithAgentRunWorkingDir(dir string) AgentRunOption {
	return func(s *agentRunController) {
		s.dir = dir
	}
}

func WithAgentRunSinkOptions(options ...sink.Option) AgentRunOption {
	return func(s *agentRunController) {
		s.sinkOptions = options
	}
}

func WithAgentRunExecOptions(options ...exec.Option) AgentRunOption {
	return func(s *agentRunController) {
		s.execOptions = options
	}
}

func WithAgentRunConsoleToken(token string) AgentRunOption {
	return func(s *agentRunController) {
		s.consoleToken = token
	}
}
