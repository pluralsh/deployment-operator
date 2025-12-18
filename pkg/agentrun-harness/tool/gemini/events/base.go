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
		// Ignored as there is no special handling needed for init events currently.
	case EventTypeMessage:
		return handleEvent[MessageEvent](line, func(e *MessageEvent) { e.Append() })
	case EventTypeToolUse:
		return handleEvent[ToolUseEvent](line, func(e *ToolUseEvent) { e.Save() })
	case EventTypeToolResult:
		return handleEvent[ToolResultEvent](line, func(e *ToolResultEvent) { onMessage(e.Attributes()) })
	case EventTypeError:
		return handleEvent[ErrorEvent](line, func(e *ErrorEvent) { onMessage(e.Attributes()) })
	case EventTypeResult:
		return handleEvent[ResultEvent](line, func(e *ResultEvent) { onMessage(e.Attributes()) })
	default:
		klog.V(log.LogLevelDebug).InfoS("ignoring unknown event", "type", e.Type, "line", string(line))
	}

	return nil
}

// handleEvent is a generic helper to unmarshal, validate and process an event.
func handleEvent[T any, PT interface {
	*T
	Validate() bool
}](line []byte, handler func(PT)) error {
	var t T
	pt := PT(&t)
	if err := json.Unmarshal(line, pt); err != nil {
		return fmt.Errorf("failed to unmarshal %T: %w", pt, err)
	}

	if !pt.Validate() {
		klog.V(log.LogLevelDebug).InfoS("ignoring invalid event", "event", pt)
		return nil
	}

	handler(pt)

	return nil
}
