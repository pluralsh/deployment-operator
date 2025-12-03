package events

import (
	"encoding/json"
	"fmt"
	"time"

	console "github.com/pluralsh/console/go/client"
	"github.com/pluralsh/deployment-operator/pkg/log"
	"k8s.io/klog/v2"
)

type EventType string

const (
	EventTypeInit       EventType = "init"
	EventTypeMessage    EventType = "message"
	EventTypeToolUse    EventType = "tool_use"
	EventTypeToolResult EventType = "tool_result"
	EventTypeError      EventType = "error"
	EventTypeResult     EventType = "result"
)

type EventBase struct {
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func (e EventBase) OnMessage(line []byte, onMessage func(message *console.AgentMessageAttributes)) error {
	if onMessage == nil {
		klog.V(log.LogLevelDebug).InfoS("ignoring event as message handler is not defined",
			"type", e.Type, "line", string(line))
		return nil
	}

	switch e.Type {
	case EventTypeInit:
		return nil // TODO
	case EventTypeMessage:
		message := &MessageEvent{}
		if err := json.Unmarshal(line, message); err != nil {
			return fmt.Errorf("failed to unmarshal Gemini message event: %w", err)
		}

		if !message.IsValid() {
			klog.V(log.LogLevelDebug).InfoS("ignoring invalid Gemini message", "message", message)
		}

		onMessage(message.Attributes())
		return nil
	case EventTypeToolUse:
		return nil // TODO
	case EventTypeToolResult:
		return nil // TODO
	case EventTypeError:
		return nil // TODO
	case EventTypeResult:
		return nil // TODO
	default:
		klog.V(log.LogLevelDebug).InfoS("ignoring Gemini event", "type", e.Type, "line", string(line))
		return nil
	}
}
