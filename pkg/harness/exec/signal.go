package exec

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/log"
)

type Signal interface {
	Listen(cancelFunc context.CancelFunc)
}

type remoteSignal struct{}

func (in *remoteSignal) Listen(cancelFunc context.CancelFunc) {
	// TODO: subscribe to console and wait for cancel event
	panic("not implemented")
}

func NewRemoteSignal() Signal {
	return &remoteSignal{}
}

type timeoutSignal struct {
	timeout time.Duration
}

func (in *timeoutSignal) Listen(cancelFunc context.CancelFunc) {
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

func NewTimeoutSignal(timeout time.Duration) Signal {
	return &timeoutSignal{
		timeout,
	}
}
