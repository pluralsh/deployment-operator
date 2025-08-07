package helpers

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
)

func PollUntilContextCancel(ctx context.Context, getInterval func() time.Duration, immediate bool, condition wait.ConditionWithContextFunc) error {
	return loopConditionUntilContext(ctx, NewDynamicTimer(getInterval), immediate, false, condition)
}

// BackgroundPollUntilContextCancel spawns a new goroutine that runs the condition function on interval.
// If syncFirstRun is set to true, it will execute the condition function synchronously first and then start
// polling. Since error is returned synchronously, the only way to check for it is to use syncFirstRun.
// Background poller does not sync errors. It can be stopped externally by cancelling the provided context.
func BackgroundPollUntilContextCancel(ctx context.Context, getInterval func() time.Duration, immediate, syncFirstRun bool, condition wait.ConditionWithContextFunc) (err error) {
	if syncFirstRun {
		_, err = condition(ctx)
	}

	go func() {
		timer := NewDynamicTimer(getInterval)
		_ = loopConditionUntilContext(ctx, timer, immediate, false, condition)
	}()

	return err
}

// loopConditionUntilContext executes the provided condition at intervals defined by
// the provided timer until the provided context is cancelled, the condition returns
// true, or the condition returns an error. If sliding is true, the period is computed
// after condition runs. If it is false then period includes the runtime for condition.
// If immediate is false the first delay happens before any call to condition, if
// immediate is true the condition will be invoked before waiting and guarantees that
// the condition is invoked at least once, regardless of whether the context has been
// cancelled. The returned error is the error returned by the last condition or the
// context error if the context was terminated.
//
// This is the common loop construct for all polling in the wait package.
func loopConditionUntilContext(ctx context.Context, t wait.Timer, immediate, sliding bool, condition wait.ConditionWithContextFunc) error {
	defer t.Stop()

	var timeCh <-chan time.Time
	doneCh := ctx.Done()

	if !sliding {
		timeCh = t.C()
	}

	// if immediate is true the condition is
	// guaranteed to be executed at least once,
	// if we haven't requested immediate execution, delay once
	if immediate {
		if ok, err := func() (bool, error) {
			defer runtime.HandleCrashWithContext(ctx)
			return condition(ctx)
		}(); err != nil || ok {
			return err
		}
	}

	if sliding {
		timeCh = t.C()
	}

	for {

		// Wait for either the context to be cancelled or the next invocation be called
		select {
		case <-doneCh:
			return ctx.Err()
		case <-timeCh:
		}

		// IMPORTANT: Because there is no channel priority selection in golang
		// it is possible for very short timers to "win" the race in the previous select
		// repeatedly even when the context has been canceled.  We therefore must
		// explicitly check for context cancellation on every loop and exit if true to
		// guarantee that we don't invoke condition more than once after context has
		// been cancelled.
		if err := ctx.Err(); err != nil {
			return err
		}

		if !sliding {
			t.Next()
		}
		if ok, err := func() (bool, error) {
			defer runtime.HandleCrashWithContext(ctx)
			return condition(ctx)
		}(); err != nil || ok {
			return err
		}
		if sliding {
			t.Next()
		}
	}
}

// DynamicTimer implements wait.Timer interface and allows dynamic polling intervals.
// When interval is 0, the timer becomes inactive (loop won't proceed).
type DynamicTimer struct {
	getInterval func() time.Duration
	timer       *time.Timer
	ch          chan time.Time
	mu          sync.Mutex
}

// NewDynamicTimer creates a new DynamicTimer with the given interval getter.
func NewDynamicTimer(getInterval func() time.Duration) *DynamicTimer {
	dt := &DynamicTimer{
		getInterval: getInterval,
		ch:          make(chan time.Time, 1),
	}
	dt.reset()
	return dt
}

// C returns the channel that fires when the timer expires.
// Even when interval is 0, this still returns a valid (but inactive) channel.
func (dt *DynamicTimer) C() <-chan time.Time {
	return dt.ch
}

// Stop stops the internal timer, if active.
func (dt *DynamicTimer) Stop() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	if dt.timer != nil {
		dt.timer.Stop()
	}
}

// Next resets the timer with the current interval from getInterval.
// If interval is 0, it disables the timer (no tick will be sent).
func (dt *DynamicTimer) Next() {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.reset()
}

// reset stops the current timer and creates a new one based on the latest interval.
// If interval is 0, the timer is not started and remains inactive.
func (dt *DynamicTimer) reset() {
	if dt.timer != nil {
		dt.timer.Stop()
	}

	interval := dt.getInterval()
	if interval <= 0 {
		dt.timer = nil
		return
	}

	dt.timer = time.AfterFunc(interval, func() {
		select {
		case dt.ch <- time.Now():
		default:
		}
	})
}
