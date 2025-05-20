package terraform

import (
	toolv1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

// Terraform implements tool.Tool interface.
type Terraform struct {
	toolv1.DefaultTool

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

	// parallelism is the number of concurrent operations to run
	parallelism *int64

	// refresh is whether to refresh the state
	refresh *bool
}
