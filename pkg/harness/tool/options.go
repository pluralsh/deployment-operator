package tool

import (
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

type Option func(v1.Tool)

func WithDir(dir string) Option {
	return func(tool v1.Tool) {

	}
}