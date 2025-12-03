package gemini

import (
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

type Model string

const (
	ModelGemini3ProPreview Model = "gemini-3-pro-preview"
	ModelGemini25Pro       Model = "gemini-2.5-pro"
	ModelGemini25Flash     Model = "gemini-2.5-flash"
	ModelGemini25FlashLite Model = "gemini-2.5-flash-lite"
	ModelGemini20Flash     Model = "gemini-2.0-flash"
	ModelGemini20FlashLite Model = "gemini-2.0-flash-lite"
)

func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvOpenCodeModel, string(ModelGemini25Pro)) {
	case string(ModelGemini3ProPreview):
		return ModelGemini3ProPreview
	case string(ModelGemini25Flash):
		return ModelGemini25Flash
	case string(ModelGemini25FlashLite):
		return ModelGemini25FlashLite
	case string(ModelGemini20Flash):
		return ModelGemini20Flash
	case string(ModelGemini20FlashLite):
		return ModelGemini20FlashLite
	default:
		return ModelGemini25Pro
	}
}
