package codex

import (
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

type Model string

const (
	ModelGPT5 Model = "gpt-5"

	// Primary Codex models
	ModelGPT51Codex     Model = "gpt-5.1-codex"
	ModelGPT51CodexMini Model = "gpt-5.1-codex-mini"
	ModelCodexMini      Model = "codex-mini-latest"

	// Optional powerful Codex options
	ModelGPT52Codex Model = "gpt-5.2-codex"

	defaultModel = ModelGPT5
)

// DefaultModel returns a sensible default
func DefaultModel() Model {
	envVal := helpers.GetEnv(controller.EnvCodexModel, string(defaultModel))
	m := Model(envVal)

	switch m {
	case ModelGPT5,
		ModelGPT51Codex,
		ModelGPT51CodexMini,
		ModelCodexMini,
		ModelGPT52Codex:
		return m
	default:
		return defaultModel
	}
}
