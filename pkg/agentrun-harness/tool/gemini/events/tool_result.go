package events

type ToolResultEvent struct {
	EventBase
	ToolID string  `json:"tool_id"`
	Status Status  `json:"status"`
	Output *string `json:"output,omitempty"`
	Error  *Error  `json:"error,omitempty"`
}
