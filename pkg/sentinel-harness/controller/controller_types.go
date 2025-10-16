package controller

import (
	"context"

	console "github.com/pluralsh/deployment-operator/pkg/client"
)

type Controller interface {
	Start(ctx context.Context) error
}

type sentinelRunController struct {
	sentinelRunID string

	// consoleClient
	consoleClient console.Client

	dir string

	outputFormat string

	// consoleToken
	consoleToken string
}

type Option func(*sentinelRunController)
