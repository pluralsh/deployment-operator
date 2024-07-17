package watcher

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

// watcherReference tracks [watch.Interface] lifecycle.
type watcherReference struct {
	// lock guards the subsequent stateful fields
	lock sync.Mutex

	context context.Context
	cancel  context.CancelFunc
	started bool

	watcher watch.Interface
}

// Start returns a wrapped context that can be cancelled.
// Returns nil & false if already started.
func (ir *watcherReference) Start(ctx context.Context) (context.Context, bool) {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	if ir.started {
		return nil, false
	}

	ctx, cancel := context.WithCancel(ctx)
	ir.context = ctx
	ir.cancel = cancel
	ir.started = true

	return ctx, true
}

func (ir *watcherReference) SetInformer(watcher watch.Interface) {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	ir.watcher = watcher
}

func (ir *watcherReference) HasSynced() bool {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	if !ir.started {
		return false
	}

	if ir.watcher == nil {
		return false
	}

	return true
}

func (ir *watcherReference) HasStarted() bool {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	return ir.started
}

// Stop cancels the context, if it's been started.
func (ir *watcherReference) Stop() {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	if !ir.started {
		return
	}

	if ir.watcher != nil {
		ir.watcher.Stop()
	}
	ir.cancel()
	ir.started = false
	ir.context = nil
}

// Restart restarts the watcher.
func (ir *watcherReference) Restart() {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	if !ir.started {
		return
	}

	if ir.watcher != nil {
		ir.watcher.Stop()
	}

	ir.started = false
	ir.context = nil
}

// Restart ...
func (ir *watcherReference) Restart() {
	ir.lock.Lock()
	defer ir.lock.Unlock()

	if !ir.started {
		return
	}

	if ir.watcher != nil {
		ir.watcher.Stop()
	}

	ir.started = false
	ir.context = nil
}
