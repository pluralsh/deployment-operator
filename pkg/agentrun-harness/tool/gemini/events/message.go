package events

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
)

var messageBuilder strings.Builder

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

func (e *MessageEvent) Validate() bool {
	return e.Type == EventTypeMessage && e.Content != ""
}

func (e *MessageEvent) Append() {
	if e.Delta != nil && *e.Delta {
		messageBuilder.WriteString(e.Content)
	}
}
