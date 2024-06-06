package ansible

// Ansible implements tool.Tool interface.
type Ansible struct {
	// dir
	dir string

	// planFileName
	planFileName string

	// planFilePath
	planFilePath string
}
