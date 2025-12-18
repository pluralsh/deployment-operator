package events

import console "github.com/pluralsh/console/go/client"

type InitEvent struct {
	EventBase
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
}

func (e *InitEvent) Validate() bool {
	return e.Type == EventTypeInit
}

func (e *InitEvent) Process(_ func(message *console.AgentMessageAttributes)) {}
