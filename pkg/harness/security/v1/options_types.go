package v1

// TerraformScanOptions TODO
type TerraformScanOptions struct {
	// Dir is a directory containing files that should be scanned.
	Dir string

	// PlanFileName is a terraform plan file name.
	PlanFileName string

	// VariablesFileName is a terraform variables file name.
	VariablesFileName string
}

// ScanOptions TODO
type ScanOptions struct {
	// Terraform TODO
	Terraform TerraformScanOptions
}

// ScanOption TODO
type ScanOption func(*ScanOptions)

// WithTerraform TODO
func WithTerraform(options TerraformScanOptions) ScanOption {
	return func(o *ScanOptions) {
		o.Terraform = options
	}
}
