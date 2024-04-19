package exec

type Command string

// Supported commands for the execution
const (
	CommandTerraform = Command("terraform")
	CommandAnsible   = Command("ansible")
)

type Executable struct {
	// workingDirectory specifies the working workingDirectory of the command.
	// If workingDirectory is empty then runs the command in the calling process's current workingDirectory.
	workingDirectory string

	// command TODO
	command Command
}

type ExecutableOption func(*Executable)
