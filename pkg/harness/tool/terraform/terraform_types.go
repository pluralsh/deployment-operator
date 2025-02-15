package terraform

// Terraform implements tool.Tool interface.
type Terraform struct {
	// dir is a working directory used by harness.
	dir string

	// planFileName is a terraform plan file name.
	// Default: terraform.tfplan
	planFileName string

	// variablesFileName is a terraform variables file name.
	// Default: plural.auto.tfvars.json
	variablesFileName string

	// variables is a JSON encoded string representing
	// terraform variable file.
	variables *string
}
