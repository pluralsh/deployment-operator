package helpers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pluralsh/deployment-operator/internal/metrics"
)

// BackgroundPollUntilContextCancel spawns a new goroutine that runs the condition function on interval.
// If syncFirstRun is set to true, it will execute the condition function synchronously first and then start
// polling. Since error is returned synchronously, the only way to check for it is to use syncFirstRun.
// Background poller does not sync errors. It can be stopped externally by cancelling the provided context.
func BackgroundPollUntilContextCancel(ctx context.Context, interval time.Duration, immediate, syncFirstRun bool, condition wait.ConditionWithContextFunc) (err error) {
	if syncFirstRun {
		metrics.Record().DiscoveryAPICacheRefresh()
		_, err = condition(ctx)
	}

	go func() {
		_ = wait.PollUntilContextCancel(ctx, interval, immediate, condition)
	}()

	return err
}
