package gemini

import (
	"time"

	console "github.com/pluralsh/console/go/client"
)

// Types defined in this file reflect ones defined in:
// https://github.com/google-gemini/gemini-cli/blob/main/packages/core/src/output/types.ts

type EventType string

const (
	EventTypeInit       EventType = "init"
	EventTypeMessage    EventType = "message"
	EventTypeToolUse    EventType = "tool_use"
	EventTypeToolResult EventType = "tool_result"
	EventTypeError      EventType = "error"
	EventTypeResult     EventType = "result"
)

type Role string

func (r Role) Attributes() console.AiRole {
	switch r {
	case RoleAssistant:
		return console.AiRoleAssistant
	case RoleUser:
		return console.AiRoleUser
	default:
		return console.AiRoleSystem
	}
}

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ErrorSeverity string

const (
	ErrorSeverityWarning ErrorSeverity = "warning"
	ErrorSeverityError   ErrorSeverity = "error"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type BaseJsonStreamEvent struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

type InitEvent struct {
	BaseJsonStreamEvent
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

type MessageEvent struct {
	BaseJsonStreamEvent
	Role    Role   `json:"role"`
	Content string `json:"content"`
	Delta   *bool  `json:"delta,omitempty"`
}

func (e *MessageEvent) IsValid() bool {
	return e.Type == EventTypeMessage && e.Content != ""
}

func (e *MessageEvent) Attributes() *console.AgentMessageAttributes {
	return &console.AgentMessageAttributes{
		Message: e.Content,
		Role:    e.Role.Attributes(),
	}
}

type ToolUseEvent struct {
	BaseJsonStreamEvent
	ToolName   string         `json:"tool_name"`
	ToolID     string         `json:"tool_id"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type ToolResultEvent struct {
	BaseJsonStreamEvent
	ToolID string  `json:"tool_id"`
	Status Status  `json:"status"`
	Output *string `json:"output,omitempty"`
	Error  *Error  `json:"error,omitempty"`
}

type ErrorEvent struct {
	BaseJsonStreamEvent
	Severity ErrorSeverity `json:"severity"`
	Message  string        `json:"message"`
}

type ResultEvent struct {
	BaseJsonStreamEvent
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
