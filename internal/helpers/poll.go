package helpers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

// DynamicBackgroundPollUntilContextCancel spawns a new goroutine that runs the condition function on interval.
// If syncFirstRun is set to true, it will execute the condition function synchronously first and then start
// polling. Since error is returned synchronously, the only way to check for it is to use syncFirstRun.
// Background poller does not sync errors. It can be stopped externally by cancelling the provided context.
func DynamicBackgroundPollUntilContextCancel(ctx context.Context, getInterval func() time.Duration, immediate, syncFirstRun bool, condition wait.ConditionWithContextFunc) (err error) {
	if syncFirstRun {
		_, err = condition(ctx)
	}

	go func() {
		_ = DynamicPollUntilContextCancel(ctx, getInterval, immediate, condition)
	}()

	return err
}

func DynamicPollUntilContextCancel(ctx context.Context, intervalFunc func() time.Duration, immediate bool, condition wait.ConditionWithContextFunc) error {
	started := false
	lastInterval := intervalFunc()

	return wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		klog.V(5).Info("running global poll")
		interval := intervalFunc()
		if lastInterval == interval && started {
			klog.V(5).Info("global - interval not changed, returning", "interval", interval)
			return false, nil
		}

		klog.V(5).Info("global - interval changed, restarting", "lastInterval", lastInterval, "interval", interval)
		started = true
		lastInterval = interval

		go func() {
			_ = wait.PollUntilContextCancel(ctx, interval, immediate, func(ctx context.Context) (bool, error) {
				klog.V(5).Info("running local poll")
				curInterval := intervalFunc()

				// Stop polling entirely if interval is 0
				if interval == 0 {
					klog.V(5).Info("global - interval set to 0, stopping poller")
					return true, nil
				}
				if interval != curInterval {
					klog.V(5).Info("local - interval changed, restarting", "curInterval", curInterval, "interval", interval)
					return true, nil
				}

				klog.V(5).Info("running condition")
				return condition(ctx)
			})
		}()
		return false, nil
	})
}
