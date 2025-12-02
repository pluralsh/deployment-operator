package gemini

import "time"

type StreamEvent struct {
	Type      string       `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	SessionID *string      `json:"session_id"`
	Model     *string      `json:"model"`
	Role      *string      `json:"role"`
	Content   *string      `json:"content"`
	Delta     *string      `json:"delta"`
	ToolName  *string      `json:"tool_name"`
	ToolID    *string      `json:"tool_id"`
	Status    *string      `json:"status"`
	Output    *string      `json:"output"`
	Stats     *StreamStats `json:"stats"`
}

type StreamStats struct {
	TotalTokens  int `json:"total_tokens"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	DurationMs   int `json:"duration_ms"`
	ToolCalls    int `json:"tool_calls"`
}
