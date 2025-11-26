package claude

import (
	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/helpers"
)

type Model string

const (
	ClaudeOpus45  Model = "claude-opus-4-5-20251101"
	ClaudeSonnet4 Model = "claude-sonnet-4-20250514"
	ClaudeOpus4   Model = "claude-opus-4-20250514"
	ClaudeOpus41  Model = "claude-opus-4-1-20250805"
)

func DefaultModel() Model {
	switch helpers.GetEnv(controller.EnvClaudeModel, string(ClaudeOpus45)) {
	case string(ClaudeOpus45):
		return ClaudeOpus45
	case string(ClaudeSonnet4):
		return ClaudeSonnet4
	case string(ClaudeOpus4):
		return ClaudeOpus4
	case string(ClaudeOpus41):
		return ClaudeOpus41
	default:
		return ClaudeOpus45
	}
}
