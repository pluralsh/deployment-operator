package exec

import (
	"time"

	"golang.org/x/net/context"
	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

type remoteCancelContext struct {
	context.Context
}

type Signal interface {
	Listen(cancelFunc context.CancelFunc)
}

type remoteCancelSignal struct{}

func (in *remoteCancelSignal) Listen(cancelFunc context.CancelFunc) {
	// TODO: subscribe to console and wait for cancel event
	panic("not implemented")
}

func NewRemoteCancelSignal() Signal {
	return &remoteCancelSignal{}
}

type timeoutCancelSignal struct {
	timeout time.Duration
}

func (in *timeoutCancelSignal) Listen(cancelFunc context.CancelFunc) {
	timer := time.NewTimer(in.timeout)

	go func() {
		select {
		case <-timer.C:
			timer.Stop()
			klog.V(log.LogLevelMinimal).InfoS("signal: timed out", "timeout", in.timeout)
			cancelFunc()
		}
	}()
}

func NewTimeoutCancelSignal(timeout time.Duration) Signal {
	return &timeoutCancelSignal{
		timeout,
	}
}

func WithCancel(parent context.Context, signals ...Signal) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)
	for _, signal := range signals {
		signal.Listen(cancel)
	}

	return &remoteCancelContext{
		Context: ctx,
	}
}
