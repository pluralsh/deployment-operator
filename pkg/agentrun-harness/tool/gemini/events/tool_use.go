package events

type ToolUseEvent struct {
	EventBase
	ToolName   string         `json:"tool_name"`
	ToolID     string         `json:"tool_id"`
	Parameters map[string]any `json:"parameters,omitempty"`
}
