package watcher

import (
	"context"
	"sync"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/cli-utils/pkg/object"
)

type taskFunc func()

// taskManager manages a set of tasks with object identifiers.
// This makes starting and stopping the tasks thread-safe.
type taskManager struct {
	lock        sync.Mutex
	cancelFuncs map[object.ObjMetadata]context.CancelFunc
}

func (tm *taskManager) Schedule(parentCtx context.Context, id object.ObjMetadata, delay time.Duration, task taskFunc) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	if tm.cancelFuncs == nil {
		tm.cancelFuncs = make(map[object.ObjMetadata]context.CancelFunc)
	}

	cancel, found := tm.cancelFuncs[id]
	if found {
		// Cancel the existing scheduled task and replace it.
		cancel()
	}

	taskCtx, cancel := context.WithTimeout(context.Background(), delay)
	tm.cancelFuncs[id] = cancel

	go func() {
		klog.V(5).Infof("Task scheduled (%v) for object (%s)", delay, id)
		select {
		case <-parentCtx.Done():
			// stop waiting
			cancel()
		case <-taskCtx.Done():
			if taskCtx.Err() == context.DeadlineExceeded {
				klog.V(5).Infof("Task executing (after %v) for object (%v)", delay, id)
				task()
			}
			// else stop waiting
		}
	}()
}

func (tm *taskManager) Cancel(id object.ObjMetadata) {
	tm.lock.Lock()
	defer tm.lock.Unlock()

	cancelFunc, found := tm.cancelFuncs[id]
	if !found {
		// already cancelled or not added
		return
	}
	delete(tm.cancelFuncs, id)
	cancelFunc()
	if len(tm.cancelFuncs) == 0 {
		tm.cancelFuncs = nil
	}
}
