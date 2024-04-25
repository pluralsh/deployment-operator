package controller

func WithExecutionStrategy(strategy ExecutionStrategy) ExecutorOption {
	return func(e *executor) {
		e.strategy = strategy
	}
}
