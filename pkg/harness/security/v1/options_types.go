package v1

type TerraformScanOptions struct {
	// Dir is a directory containing files that should be scanned.
	Dir string

	// PlanFileName is a terraform plan file name.
	PlanFileName string

	// VariablesFileName is a terraform variables file name.
	VariablesFileName string
}

type ScanOptions struct {
	// Terraform TODO
	Terraform TerraformScanOptions
}

type ScanOption func(*ScanOptions)

func WithTerraform(options TerraformScanOptions) ScanOption {
	return func(o *ScanOptions) {
		o.Terraform = options
	}
}