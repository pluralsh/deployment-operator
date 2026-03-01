package codex

import (
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

type Model string

const (
	// Primary Codex models
	ModelGPT51Codex     Model = "gpt-5.1-codex"
	ModelGPT51CodexMini Model = "gpt-5.1-codex-mini"
	ModelCodexMini      Model = "codex-mini-latest"

	// Optional powerful Codex options
	ModelGPT52Codex Model = "gpt-5.2-codex"
)

// DefaultModel returns a sensible default
func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvCodexModel, string(ModelGPT51Codex)) {
	case string(ModelGPT51Codex):
		return ModelGPT51Codex
	case string(ModelGPT51CodexMini):
		return ModelGPT51CodexMini
	case string(ModelCodexMini):
		return ModelCodexMini
	case string(ModelGPT52Codex):
		return ModelGPT52Codex
	default:
		return ModelGPT51Codex
	}
}
