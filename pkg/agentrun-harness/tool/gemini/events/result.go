package events

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

type StreamStats struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	DurationMs   int `json:"duration_ms"`
	ToolCalls    int `json:"tool_calls"`
}

func (s *StreamStats) Attributes() *console.AgentMessageCostAttributes {
	if s == nil {
		return nil
	}

	return &console.AgentMessageCostAttributes{
		Total: float64(s.TotalTokens),
		Tokens: &console.AgentMessageTokensAttributes{
			Input:  lo.ToPtr(float64(s.InputTokens)),
			Output: lo.ToPtr(float64(s.OutputTokens)),
		},
	}
}

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

type ResultEvent struct {
	EventBase
	Status Status       `json:"status"`
	Error  *ResultError `json:"error,omitempty"`
	Stats  *StreamStats `json:"stats,omitempty"`
}

func (e *ResultEvent) Validate() bool {
	return e.Type == EventTypeResult &&
		((e.Status == StatusSuccess && messageBuilder.Len() > 0) || e.Status == StatusError)
}

func (e *ResultEvent) Process(onMessage func(message *console.AgentMessageAttributes)) {
	onMessage(&console.AgentMessageAttributes{
		Message: messageBuilder.String(),
		Role:    console.AiRoleAssistant,
		Cost:    e.Stats.Attributes(),
	})
}

type ResultError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
