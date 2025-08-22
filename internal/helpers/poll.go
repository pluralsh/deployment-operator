package helpers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/apimachinery/pkg/util/wait"
)

// DynamicBackgroundPollUntilContextCancel spawns a new goroutine that runs the condition function on interval.
// If syncFirstRun is set to true, it will execute the condition function synchronously first and then start
// polling. Since error is returned synchronously, the only way to check for it is to use syncFirstRun.
// Background poller does not sync errors. It can be stopped externally by cancelling the provided context.
func DynamicBackgroundPollUntilContextCancel(ctx context.Context, getInterval func() time.Duration, syncFirstRun bool, condition wait.ConditionWithContextFunc) (err error) {
	if syncFirstRun {
		_, err = condition(ctx)
	}

	go func() {
		_ = DynamicPollUntilContextCancel(ctx, getInterval, condition)
	}()

	return err
}

func DynamicPollUntilContextCancel(
	ctx context.Context,
	intervalFunc func() time.Duration,
	condition wait.ConditionWithContextFunc,
) error {
	for {
		interval := intervalFunc()

		// Handle inactive state (interval == 0) and wait 1sec
		for interval <= 0 {
			ticker := time.NewTicker(time.Second)
			select {
			case <-ctx.Done():
				ticker.Stop()
				return ctx.Err()
			case <-ticker.C:
				interval = intervalFunc()
			}
			ticker.Stop()
		}

		// Active polling mode
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
			if ok, err := func() (bool, error) {
				defer runtime.HandleCrashWithContext(ctx)
				return condition(ctx)
			}(); err != nil || ok {
				return err
			}
		}
	}
}
