package errors

import (
	"errors"
)

var (
	ErrTimeout      = errors.New("timed out")
	ErrRemoteCancel = errors.New("cancelled remotely")
	ErrNotFound     = errors.New("resource not found")
	ErrTerminated   = errors.New("process has been terminated")
	ErrNoChanges    = errors.New("plan has no changes, skipping run")
)
