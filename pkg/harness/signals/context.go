package signals

import (
	"context"
)

type cancelableContext struct {
	context.Context
}

func NewCancelableContext(parent context.Context, signals ...Signal) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancelCause(parent)
	for _, signal := range signals {
		signal.Listen(cancel)
	}

	return &cancelableContext{
		Context: ctx,
	}
}
