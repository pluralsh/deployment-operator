package errors

import (
"errors"
)

var (
	ErrTimeout = errors.New("timed out")
	ErrRemoteCancel = errors.New("cancelled remotely")
	ErrNotFound = errors.New("resource not found")
)
