package events

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

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
	ToolID string     `json:"tool_id"`
	Status ToolStatus `json:"status"`
	Output *string    `json:"output,omitempty"`
	Error  *Error     `json:"error,omitempty"`
}

func (e *ToolResultEvent) IsValid() bool {
	return e.Type == EventTypeMessage && e.ToolID != ""
}

func (e *ToolResultEvent) Attributes() *console.AgentMessageAttributes {
	attrs := &console.AgentMessageAttributes{
		Message: IgnoreMessage,
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
