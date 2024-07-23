package signals

import (
	"context"
	"time"

	gqlclient "github.com/pluralsh/console/go/client"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	console "github.com/pluralsh/deployment-operator/pkg/client"
	"github.com/pluralsh/deployment-operator/pkg/harness/environment"
	"github.com/pluralsh/deployment-operator/pkg/harness/errors"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

type consoleSignal struct {
	client console.Client
	id     string
}

func (in *consoleSignal) Listen(cancelFunc context.CancelCauseFunc) {
	klog.V(log.LogLevelDebug).InfoS("starting console signal listener")

	ctx, cancel := context.WithCancel(context.Background())
	resyncPeriod := 5 * time.Second

	go wait.Until(func() {
		stackRun, err := in.client.GetStackRunBase(in.id)
		if err != nil {
			klog.ErrorS(err, "could not resync stack run", "id", in.id)
			return
		}

		// Allow rerunning cancelled runs when in dev mode.
		if stackRun != nil && stackRun.Status == gqlclient.StackStatusCancelled && !environment.IsDev() {
			cancelFunc(errors.ErrRemoteCancel)
			cancel()
		}
	}, resyncPeriod, ctx.Done())
}

func NewConsoleSignal(client console.Client, id string) Signal {
	return &consoleSignal{
		client,
		id,
	}
}

type timeoutSignal struct {
	timeout time.Duration
}

func (in *timeoutSignal) Listen(cancelFunc context.CancelCauseFunc) {
	klog.V(log.LogLevelDebug).InfoS("starting timeout signal listener")
	timer := time.NewTimer(in.timeout)

	go func() {
		<-timer.C
		timer.Stop()
		cancelFunc(errors.ErrTimeout)
	}()
}

func NewTimeoutSignal(timeout time.Duration) Signal {
	return &timeoutSignal{
		timeout,
	}
}
