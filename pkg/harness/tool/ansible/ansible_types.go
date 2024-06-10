package ansible

import (
	v1 "github.com/pluralsh/deployment-operator/pkg/harness/tool/v1"
)

// Ansible implements tool.Tool interface.
type Ansible struct {
	v1.DefaultTool

	// workDir
	workDir string

	// execDir
	execDir string

	// planFileName
	planFileName string

	// planFilePath
	planFilePath string
}
