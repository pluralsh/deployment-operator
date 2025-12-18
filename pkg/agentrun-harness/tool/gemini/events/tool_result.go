package events

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

const IgnoreMessage = "__plrl_ignore__"

type ToolStatus string

const (
	ToolStatusSuccess ToolStatus = "success"
	ToolStatusError   ToolStatus = "error"
)

func (s ToolStatus) Attributes() *console.AgentMessageToolState {
	switch s {
	case ToolStatusSuccess:
		return lo.ToPtr(console.AgentMessageToolStateCompleted)
	case ToolStatusError:
		return lo.ToPtr(console.AgentMessageToolStateError)
	default:
		return lo.ToPtr(console.AgentMessageToolStatePending)
	}
}

type ToolResultEvent struct {
	EventBase
	ToolID string           `json:"tool_id"`
	Status ToolStatus       `json:"status"`
	Output *string          `json:"output,omitempty"`
	Error  *ToolResultError `json:"error,omitempty"`
}

func (e *ToolResultEvent) Validate() bool {
	return e.Type == EventTypeToolResult && e.ToolID != ""
}

func (e *ToolResultEvent) Process(onMessage func(message *console.AgentMessageAttributes)) {
	onMessage(e.Attributes())
}

func (e *ToolResultEvent) Attributes() *console.AgentMessageAttributes {
	attrs := &console.AgentMessageAttributes{
		Message: IgnoreMessage,
		Role:    console.AiRoleAssistant,
		Metadata: &console.AgentMessageMetadataAttributes{
			Tool: &console.AgentMessageToolAttributes{
				Name:   lo.ToPtr(e.ToolID),
				State:  e.Status.Attributes(),
				Output: e.Output,
			},
		},
	}

	if toolUse, ok := toolUseCache.Get(e.ToolID); ok {
		attrs.Metadata.Tool.Name = lo.ToPtr(toolUse.ToolName)
	}

	return attrs
}

type ToolResultError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
