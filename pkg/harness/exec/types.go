package exec

import (
	"context"
	"io"

	gqlclient "github.com/pluralsh/console-client-go"
)

type ExitCode uint8

// TODO: Figure out how to enforce these exit codes
const (
	// ExitCodeOK - successful termination
	ExitCodeOK     ExitCode = 0
	// ExitCodeUsage - command line usage error
	ExitCodeUsage  ExitCode = 64
	// ExitCodeCancel - process stopped/killed via an external signal
	ExitCodeCancel ExitCode = 65
	// ExitCodeOther - other not recognized errors
	ExitCodeOther ExitCode = 255
)

type Command string

// Supported commands for the execution
const (
	CommandTerraform = Command(gqlclient.StackTypeTerraform)
	CommandAnsible   = Command(gqlclient.StackTypeAnsible)
)

// Executable wraps command calls to make it easier to run and process output.
type Executable struct {
	// workingDirectory specifies the working workingDirectory of the command.
	// If workingDirectory is empty then runs the command in the calling process's current workingDirectory.
	workingDirectory string

	// command specifies root command that will be executed
	command Command

	// context
	context context.Context

	// cancel
	cancel func() error

	// standardLogSink
	standardLogSink io.Writer

	// errorLogSink
	errorLogSink io.Writer
}

type ExecutableOption func(*Executable)
