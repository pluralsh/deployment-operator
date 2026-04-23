package controller

import (
	console "github.com/pluralsh/deployment-operator/pkg/client"
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

func WithDeployToken(token string) Option {
	return func(s *agentRunController) {
		s.deployToken = token
	}
}

func WithConsoleUrl(url string) Option {
	return func(s *agentRunController) {
		s.consoleUrl = url
	}
}

// WithSkipInitialRun is a test helper that skips the real AI CLI execution.
// It closes runDone immediately after tool.Run() is called so the babysit
// loop starts without waiting for a real process to finish.
func WithSkipInitialRun() Option {
	return func(s *agentRunController) {
		s.skipInitialRun = true
	}
}

