package events

import (
	console "github.com/pluralsh/console/go/client"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

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

type MessageEvent struct {
	EventBase
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
