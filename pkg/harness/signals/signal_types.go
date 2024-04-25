package signals

import (
	"context"
)

type ExitCode uint8

func (e ExitCode) Int() int {
	return int(e)
}

const (
	// ExitCodeOK - successful termination
	ExitCodeOK ExitCode = 0
	// ExitCodeUsage - command line usage error
	ExitCodeUsage ExitCode = 64
	// ExitCodeCancel - process stopped/killed via an external signal
	ExitCodeCancel ExitCode = 65
	// ExitCodeTimeout - process was running for too long and timed out
	ExitCodeTimeout ExitCode = 66
	// ExitCodeOther - other not recognized errors
	ExitCodeOther ExitCode = 255
)

type Signal interface {
	Listen(cancelFunc context.CancelCauseFunc)
}
