package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	"github.com/pluralsh/deployment-operator/internal/mcpserver"
)

func init() {
	defaultFlagSet := flag.CommandLine

	// Init klog
	klog.InitFlags(defaultFlagSet)
}

func main() {
	// Load credentials
	creds, err := mcpserver.LoadPluralCredentials()
	if err != nil {
		klog.ErrorS(err,
			fmt.Sprintf("Failed to load Plural credentials, please ensure %s and %s environment variables are set",
				controller.EnvConsoleToken,
				controller.EnvConsoleURL,
			),
		)
		os.Exit(1)
	}

	// Create MCP server
	server := mcpserver.NewMCPServer(creds)

	// Start server
	if err := server.Start(); err != nil {
		klog.Fatalf("Failed to start MCP server: %v", err)
	}
}
