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
	types "github.com/pluralsh/deployment-operator/pkg/harness/signals"
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
		agentRun, err := in.client.GetAgentRun(ctx, in.id)
		if err != nil {
			klog.ErrorS(err, "could not resync agent run", "id", in.id)
			return
		}

		// Allow rerunning cancelled runs when in dev mode.
		if agentRun != nil && agentRun.Status == gqlclient.AgentRunStatusCancelled && !environment.IsDev() {
			cancelFunc(errors.ErrRemoteCancel)
			cancel()
		}
	}, resyncPeriod, ctx.Done())
}

func NewConsoleSignal(client console.Client, id string) types.Signal {
	return &consoleSignal{
		client,
		id,
	}
}
