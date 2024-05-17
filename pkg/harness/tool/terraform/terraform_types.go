package terraform

// Terraform implements tool.Tool interface.
type Terraform struct {
	// dir is a working directory used by harness.
	dir string

	// planFileName is a terraform plan file name.
	// Default: terraform.tfplan
	planFileName string

	// stateFileName is a terraform state file name.
	// Default: terraform.tfstate
	stateFileName string
}
