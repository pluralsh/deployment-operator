package v1

import (
	console "github.com/pluralsh/console/go/client"
)

type ScannerType string

const (
	ScannerTypeTrivy ScannerType = "trivy"
)

type Scanner interface {
	Scan(tool console.StackType, options ...ScanOption) (json string, err error)
}

type DefaultScanner struct {
	// PolicyPaths TODO
	PolicyPaths []string
}
