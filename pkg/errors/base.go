package errors

import (
	"errors"
)

var ExpectedError = errors.New("this is a transient, expected error")
