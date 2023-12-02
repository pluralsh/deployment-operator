package errors

import (
	"fmt"
)

var UnauthenticatedError = fmt.Errorf("This agent cannot access the plural api, %w", ExpectedError)
var TransientManifestError = fmt.Errorf("This is a temporary api error, %w", ExpectedError)
