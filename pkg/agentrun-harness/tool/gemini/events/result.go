package events

type ResultEvent struct {
	EventBase
	Status Status       `json:"status"`
	Error  *Error       `json:"error,omitempty"`
	Stats  *StreamStats `json:"stats,omitempty"`
}

type StreamStats struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	DurationMs   int `json:"duration_ms"`
	ToolCalls    int `json:"tool_calls"`
}
