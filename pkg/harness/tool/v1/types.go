package v1

import (
	console "github.com/pluralsh/console-client-go"
)

// Tool handles one of the supported infrastructure management tools.
// List of supported tools is based on the gqlclient.StackType.
// It is mainly responsible for:
// - gathering state/plan information after successful run from local files
// - gathering any available outputs from local files
// - providing runtime modifiers to alter step command execution arguments, etc.
type Tool interface {
	// State tries to assemble state/plan information based on local files
	// created by specific tool after all steps are finished running. It then
	// transforms this information into gqlclient.StackStateAttributes.
	State() (*console.StackStateAttributes, error)
	// Output tries to find any available outputs information based on local files
	// created by specific tool after all steps are finished running. It then
	// transforms this information into gqlclient.StackOutputAttributes.
	Output() ([]*console.StackOutputAttributes, error)
	// Modifier returns specific modifier implementation based on the
	// current step stage. Modifiers can for example alter arguments of the
	// executable step command.
	Modifier(stage console.StepStage) Modifier
}

// Modifier can do many different runtime modifications
// of the provided stack run steps. For example, it can
// alter arguments of the executable step command.
type Modifier interface {
	// Args implements exec.ArgsModifier type.
	Args(args []string) []string
}
