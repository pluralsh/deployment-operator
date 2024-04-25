package signals

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

type consoleSignal struct {
	client console.Client
}

func (in *consoleSignal) Listen(cancelFunc context.CancelCauseFunc) {
	// TODO: subscribe to console and wait for cancel event
	klog.V(log.LogLevelDebug).InfoS("starting console signal listener")
}

func NewConsoleSignal(client console.Client) Signal {
	return &consoleSignal{
		client,
	}
}

type timeoutSignal struct {
	timeout time.Duration
}

func (in *timeoutSignal) Listen(cancelFunc context.CancelCauseFunc) {
	klog.V(log.LogLevelDebug).InfoS("starting timeout signal listener")
	timer := time.NewTimer(in.timeout)

	go func() {
		select {
		case <-timer.C:
			timer.Stop()
			cancelFunc(errors.ErrTimeout)
		}
	}()
}

func NewTimeoutSignal(timeout time.Duration) Signal {
	return &timeoutSignal{
		timeout,
	}
}
