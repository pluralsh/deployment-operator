package errors

import (
	"fmt"
)

var ErrUnauthenticated = fmt.Errorf("This agent cannot access the plural api, %w", ErrExpected)
var ErrTransientManifest = fmt.Errorf("This is a temporary api error, %w", ErrExpected)
