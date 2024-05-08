package terraform

// InitModifier implements v1.Modifier interface.
type InitModifier struct {}

// PlanModifier implements v1.Modifier interface.
type PlanModifier struct {
	// planFileName
	planFileName string
}

// ApplyModifier implements tool.Modifier interface.
type ApplyModifier struct {
	// planFileName
	planFileName string
}
