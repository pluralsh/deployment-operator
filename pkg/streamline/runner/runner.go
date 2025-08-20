package runner

import (
	"context"
	"fmt"
	"sync"
)

type TaskID string

type Task struct {
	ID        TaskID
	Do        func(ctx context.Context) error
	DependsOn []TaskID
}

type taskNode struct {
	task     *Task
	children []*taskNode
	parents  int
}

type Runner struct {
	tasks map[TaskID]*taskNode
	mu    sync.Mutex
}

func NewRunner() *Runner {
	return &Runner{
		tasks: make(map[TaskID]*taskNode),
	}
}

func (r *Runner) AddTask(t *Task) error {
	if _, exists := r.tasks[t.ID]; exists {
		return fmt.Errorf("duplicate task ID: %s", t.ID)
	}
	node := &taskNode{task: t}
	r.tasks[t.ID] = node
	return nil
}

func (r *Runner) BuildGraph() error {
	// Connect dependencies
	for _, node := range r.tasks {
		for _, dep := range node.task.DependsOn {
			parent, ok := r.tasks[dep]
			if !ok {
				return fmt.Errorf("missing dependency: %s", dep)
			}
			parent.children = append(parent.children, node)
			node.parents++
		}
	}
	return nil
}

func (r *Runner) Run(ctx context.Context, maxWorkers int) error {
	var workerWG sync.WaitGroup // wait for workers to exit
	var taskWG sync.WaitGroup   // wait for *tasks* to finish
	taskWG.Add(len(r.tasks))    // we know how many tasks we have

	taskCh := make(chan *taskNode)
	errCh := make(chan error, 1)

	// ---------- workers ----------
	for i := 0; i < maxWorkers; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for node := range taskCh { // exits when taskCh is finally closed
				if err := node.task.Do(ctx); err != nil {
					select {
					case errCh <- fmt.Errorf("task %s failed: %w", node.task.ID, err):
					default:
					}
					taskWG.Done()
					continue
				}
				r.markDone(node, taskCh) // may enqueue more tasks
				taskWG.Done()            // this task is finished
			}
		}()
	}

	// ---------- enqueue root nodes ----------
	for _, n := range r.tasks {
		if n.parents == 0 {
			taskCh <- n
		}
	}

	// ---------- close the channel *after* the last task ----------
	go func() {
		taskWG.Wait() // blocks until every taskWG.Done() has been called
		close(taskCh) // safe: no more sends will happen after this
	}()

	// ---------- wait for result ----------
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-func() chan struct{} { ch := make(chan struct{}); go func() { workerWG.Wait(); close(ch) }(); return ch }():
		return nil
	}
}

func (r *Runner) markDone(node *taskNode, taskCh chan *taskNode) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, child := range node.children {
		child.parents--
		if child.parents == 0 {
			taskCh <- child // safe: channel is still open until taskWG.Wait() completes
		}
	}
}
