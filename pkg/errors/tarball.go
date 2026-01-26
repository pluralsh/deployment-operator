package errors

import (
	"fmt"
	"strings"
)

const DigestMismatchErrorPrefix = "tarball sha mismatch"

func NewDigestMismatchError(expected, actual string) error {
	return fmt.Errorf("%s: expected %s, actual %s", DigestMismatchErrorPrefix, expected, actual)
}

func IsDigestMismatchError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), DigestMismatchErrorPrefix)
}
