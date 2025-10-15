package controller

import (
	console "github.com/pluralsh/deployment-operator/pkg/client"
)

func WithSentinelRun(id string) Option {
	return func(s *sentinelRunController) {
		s.sentinelRunID = id
	}
}

func WithConsoleClient(client console.Client) Option {
	return func(s *sentinelRunController) {
		s.consoleClient = client
	}
}

func WithWorkingDir(dir string) Option {
	return func(s *sentinelRunController) {
		s.dir = dir
	}
}

func WithConsoleToken(token string) Option {
	return func(s *sentinelRunController) {
		s.consoleToken = token
	}
}
