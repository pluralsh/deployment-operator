package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/klog/v2"

	"github.com/pluralsh/deployment-operator/internal/controller"
	agentprserver "github.com/pluralsh/deployment-operator/internal/mcpserver/agent-pr-server"
)

func init() {
	defaultFlagSet := flag.CommandLine

	// Init klog
	klog.InitFlags(defaultFlagSet)
}

func main() {
	// Load credentials
	creds, err := agentprserver.LoadPluralCredentials()
	if err != nil {
		klog.ErrorS(err,
			fmt.Sprintf("Failed to load Plural credentials, please ensure %s and %s environment variables are set",
				controller.EnvDeployToken,
				controller.EnvConsoleURL,
			),
		)
		os.Exit(1)
	}

	// Create MCP server
	server := agentprserver.NewMCPServer(creds)

	// Start server
	if err = server.Start(); err != nil {
		klog.Fatalf("Plural Console MCP server error: %v, exiting", err)
	}
}
