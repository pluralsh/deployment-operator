package events

import (
	"strings"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"k8s.io/klog/v2"
)

var messageBuilder strings.Builder

func AppendMessage(message string) {
	if messageBuilder.Len() > 0 {
		if !strings.HasSuffix(messageBuilder.String(), " ") && !strings.HasPrefix(message, " ") {
			messageBuilder.WriteString(" ")
		}
	}

	messageBuilder.WriteString(message)
}

func HasMessage() bool {
	return messageBuilder.Len() > 0
}

func GetMessage() string {
	return messageBuilder.String()
}

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
	return e.Type == EventTypeMessage && e.Content != "" && e.Delta != nil && *e.Delta
}

func (e *MessageEvent) Process(_ func(message *console.AgentMessageAttributes)) {
	AppendMessage(e.Content)
	klog.V(log.LogLevelDebug).Infof("appended message delta: %s", e.Content)
}
