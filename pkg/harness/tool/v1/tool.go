package v1

import (
	console "github.com/pluralsh/console-client-go"
)

// State implements [Tool] interface.
func (in *DefaultTool) State() (*console.StackStateAttributes, error) {
	return nil, nil
}

// Output implements [Tool] interface.
func (in *DefaultTool) Output() ([]*console.StackOutputAttributes, error) {
	return []*console.StackOutputAttributes{}, nil
}

// ConfigureStateBackend implements [Tool] interface.
func (in *DefaultTool) ConfigureStateBackend(_, _ string, _ *console.StackRunBaseFragment_StateUrls) error {
	return nil
}

// Plan implements [Tool] interface.
func (in *DefaultTool) Plan() (*console.StackStateAttributes, error) {
	return nil, nil
}

// Modifier implements [Tool] interface.
func (in *DefaultTool) Modifier(stage console.StepStage) Modifier {
	return nil
}
