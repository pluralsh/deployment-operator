package v1

import (
	console "github.com/pluralsh/console/go/client"
)

// ScannerType TODO
type ScannerType string

// Scanner TODO
type Scanner interface {
	Scan(tool console.StackType, options ...ScanOption) (violations []*console.StackPolicyViolationAttributes, err error)
}

// DefaultScanner is a base [Scanner] struct that holds shared configuration variables.
type DefaultScanner struct {
	// PolicyPaths TODO
	PolicyPaths []string

	// PolicyNamespaces TODO
	PolicyNamespaces []string
}

// Config TODO
type Config struct{}
