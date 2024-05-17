package controller

import (
	"github.com/pluralsh/deployment-operator/internal/helpers"
	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/sink"
)

func WithStackRun(id string) Option {
	return func(s *stackRunController) {
		s.stackRunID = id
	}
}

func WithConsoleClient(client console.Client) Option {
	return func(s *stackRunController) {
		s.consoleClient = client
	}
}

func WithFetchClient(client helpers.FetchClient) Option {
	return func(s *stackRunController) {
		s.fetchClient = client
	}
}

func WithWorkingDir(dir string) Option {
	return func(s *stackRunController) {
		s.dir = dir
	}
}

func WithSinkOptions(options ...sink.Option) Option {
	return func(s *stackRunController) {
		s.sinkOptions = options
	}
}
