package events

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/samber/lo"
	"k8s.io/klog/v2"
)

var toolUseCache = cmap.ConcurrentMap[string, ToolUseEvent]{}

type ToolUseEvent struct {
	EventBase
	ToolName   string         `json:"tool_name"`
	ToolID     string         `json:"tool_id"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

func (e *ToolUseEvent) IsValid() bool {
	return e.Type == EventTypeToolUse && e.ToolID != "" && e.ToolName != ""
}

func (e *ToolUseEvent) Save() {
	toolUseCache.Set(e.ToolID, lo.FromPtr(e))
	klog.Infof("saved tool use in the cache: %s", e.ToolName)
}
