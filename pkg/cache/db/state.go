package db

import (
	console "github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

type ComponentState int32

const (
	ComponentStateRunning ComponentState = iota
	ComponentStatePending
	ComponentStateFailed
	ComponentStatePaused
)

func ToComponentState(in *console.ComponentState) ComponentState {
	if in == nil {
		return ComponentStatePending
	}

	switch *in {
	case console.ComponentStateRunning:
		return ComponentStateRunning
	case console.ComponentStatePending:
		return ComponentStatePending
	case console.ComponentStateFailed:
		return ComponentStateFailed
	case console.ComponentStatePaused:
		return ComponentStatePaused
	default:
		return ComponentStatePending
	}
}

func FromComponentState(in ComponentState) *console.ComponentState {
	switch in {
	case ComponentStateRunning:
		return lo.ToPtr(console.ComponentStateRunning)
	case ComponentStatePending:
		return lo.ToPtr(console.ComponentStatePending)
	case ComponentStateFailed:
		return lo.ToPtr(console.ComponentStateFailed)
	case ComponentStatePaused:
		return lo.ToPtr(console.ComponentStatePaused)
	default:
		return nil
	}
}
