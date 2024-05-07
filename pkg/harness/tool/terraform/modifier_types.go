package terraform

// PlanModifier implements tool.Modifier interface.
type PlanModifier struct {
	// planFileName
	planFileName string
}

// ApplyModifier implements tool.Modifier interface.
type ApplyModifier struct {
	// planFileName
	planFileName string
}
