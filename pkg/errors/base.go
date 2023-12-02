package errors

import (
	"errors"
)

var ExpectedError = errors.New("This is a transient, expected error")
