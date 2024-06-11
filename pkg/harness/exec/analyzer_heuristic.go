package exec

import (
	"bufio"
	"strings"

	"github.com/pluralsh/polly/algorithms"
)

type keywordDetector struct {
	keywords []string
}

// Detect implements [OutputAnalyzerHeuristic] interface.
// TODO: we can spread actual message analysis into multiple routines to speed up the process.
func (in *keywordDetector) Detect(input *bufio.Scanner) Errors {
	line := 0
	errors := make([]Error, 0)
	for input.Scan() {
		if !in.hasError(input.Text()) {
			continue
		}

		errors = append(errors, Error{
			line:    line,
			message: input.Text(),
		})
	}

	return errors
}

func (in *keywordDetector) hasError(message string) bool {
	// do ignore case comparison
	message = strings.ToLower(message)

	return algorithms.Index(in.keywords, func(k string) bool {
		return strings.Contains(message, k)
	}) >= 0
}

func NewKeywordDetector(keywords ...string) OutputAnalyzerHeuristic {
	return &keywordDetector{
		// make sure that the default keyword strings are all lower case
		keywords: append(
			keywords,
			"error message: http remote state already locked",
			"error acquiring the state lock",
		),
	}
}
