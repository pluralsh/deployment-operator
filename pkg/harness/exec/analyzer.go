package exec

import (
	"bufio"
	"bytes"
	"strings"
)

type outputAnalyzer struct {
	output     *bytes.Buffer
	heuristics []OutputAnalyzerHeuristic
}

func (in *outputAnalyzer) Write(p []byte) (n int, err error) {
	return in.output.Write(p)
}

func (in *outputAnalyzer) Detect() []error {
	errors := make([]error, 0)
	output := in.output.String()

	for _, heuristic := range in.heuristics {
		if potentialErrors := heuristic.Detect(bufio.NewScanner(strings.NewReader(output))); len(potentialErrors) > 0 {
			errors = append(errors, potentialErrors.ToErrors()...)
		}
	}

	return errors
}

func NewOutputAnalyzer(heuristic ...OutputAnalyzerHeuristic) OutputAnalyzer {
	return &outputAnalyzer{
		output:     bytes.NewBuffer([]byte{}),
		heuristics: heuristic,
	}
}
