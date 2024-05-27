package controller

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/pkg/harness/exec"
	"github.com/pluralsh/deployment-operator/pkg/harness/stackrun"
	"github.com/pluralsh/deployment-operator/pkg/log"
)

func (in *executor) Start(ctx context.Context) error {
	in.start.Lock()
	in.started = true
	in.start.Unlock()

	switch in.strategy {
	case ExecutionStrategyOrdered:
		in.ordered(ctx)
		return nil
	case ExecutionStrategyParallel:
		in.parallel(ctx)
		return nil
	}

	return fmt.Errorf("unknown execution strategy %v", in.strategy)
}

func (in *executor) Add(executable exec.Executable) error {
	if in.started {
		return fmt.Errorf("executor has already started")
	}

	klog.V(log.LogLevelDebug).InfoS("enqueueing", "command", executable.Command())

	in.start.Lock()
	defer in.start.Unlock()
	in.startQueue = append(in.startQueue, executable)

	return nil
}

func (in *executor) ordered(ctx context.Context) {
	if len(in.startQueue) == 0 {
		klog.V(log.LogLevelDebug).InfoS("executables queue is empty", "queue", len(in.startQueue))
		return
	}

	klog.V(log.LogLevelDebug).InfoS("starting executables in order", "queue", len(in.startQueue))

	// Read executables and run them in order
	go func() {
		for _, executable := range in.startQueue {
			if err := in.run(ctx, executable); err != nil {
				in.errChan <- err
				return
			}
		}
		close(in.finishedChan)
	}()
}

func (in *executor) parallel(ctx context.Context) {
	if len(in.startQueue) == 0 {
		klog.V(log.LogLevelDebug).InfoS("executables queue is empty", "queue", len(in.startQueue))
		return
	}

	klog.V(log.LogLevelDebug).InfoS("starting executables in parallel", "queue", len(in.startQueue))

	wg := &sync.WaitGroup{}

	// Run all executables at once
	for i := range in.startQueue {
		wg.Add(1)
		executable := in.startQueue[i]
		go func() {
			if err := in.run(ctx, executable); err != nil {
				in.errChan <- err
			}
			wg.Done()
		}()
	}

	go func() {
		// We are finished when all executables complete.
		wg.Wait()
		close(in.finishedChan)
	}()
}

func (in *executor) run(ctx context.Context, executable exec.Executable) (retErr error) {
	in.preRun(executable.ID())

	if err := executable.Run(ctx); err != nil {
		retErr = fmt.Errorf("command execution failed: %s: err: %w", executable.Command(), err)
	}

	if in.postRunFunc != nil {
		in.postRunFunc(executable.ID(), retErr)
	}

	return retErr
}

func (in *executor) preRun(id string) {
	if in.preRunFunc != nil {
		in.preRunFunc(id)
	}
}

func (in *executor) runLifecycleFunction(lifecycle stackrun.Lifecycle) error {
	if fn, exists := in.hookFunctions[lifecycle]; exists {
		return fn()
	}

	return nil
}

func newExecutor(errChan chan error, finishedChan chan struct{}, options ...ExecutorOption) *executor {
	result := &executor{
		errChan:       errChan,
		finishedChan:  finishedChan,
		strategy:      ExecutionStrategyOrdered,
		ch:            make(chan exec.Executable),
		hookFunctions: make(map[stackrun.Lifecycle]stackrun.HookFunction),
	}

	for _, option := range options {
		option(result)
	}

	return result
}
