package v1

// TerraformScanOptions defines options for terraform scan.
type TerraformScanOptions struct {
	// Dir is a directory containing files that should be scanned.
	Dir string

	// PlanFileName is a terraform plan file name.
	PlanFileName string

	// VariablesFileName is a terraform variables file name.
	VariablesFileName string
}

// ScanOptions is a wrapper for tool-specific scan options.
type ScanOptions struct {
	// Terraform scan options
	Terraform TerraformScanOptions
}

// ScanOption is a function that modifies [ScanOptions].
type ScanOption func(*ScanOptions)

// WithTerraform sets Terraform scan options.
func WithTerraform(options TerraformScanOptions) ScanOption {
	return func(o *ScanOptions) {
		o.Terraform = options
	}
}
