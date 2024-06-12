package exec

import (
	"bufio"
	"fmt"
	"io"
)

// OutputAnalyzer captures the command output
// and attempts to detect potential errors.
type OutputAnalyzer interface {
	Stdout() io.Writer
	Stderr() io.Writer

	//// Write implements [io.Writer] interface.
	//Write(p []byte) (n int, err error)

	// Detect scans the output for potential errors.
	// It uses a custom heuristics to detect issues.
	// It can result in a false positives.
	//
	// Note: Make sure that it is executed after Write
	//		 has finished to ensure proper detection.
	Detect() []error
}

type OutputAnalyzerHeuristic interface {
	Detect(input *bufio.Scanner) Errors
}

type Error struct {
	line    int
	message string
}

func (in Error) ToError() error {
	return fmt.Errorf("[%d] %s", in.line, in.message)
}

type Errors []Error

func (in Errors) ToErrors() []error {
	errors := make([]error, len(in))
	for _, err := range in {
		errors = append(errors, err.ToError())
	}

	return errors
}
