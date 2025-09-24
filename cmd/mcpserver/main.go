package main

import (
	"fmt"
	"log"
	"os"

	"github.com/pluralsh/deployment-operator/internal/mcpserver"
)

func main() {
	// Load credentials
	creds, err := mcpserver.LoadPluralCredentials()
	if err != nil {
		fmt.Printf("Failed to load Plural credentials: %v\n", err)
		fmt.Println("Please ensure PLURAL_ACCESS_TOKEN and PLURAL_CONSOLE_URL environment variables are set")
		os.Exit(1)
	}

	// Create MCP server
	server := mcpserver.NewMCPServer(creds)

	// Start server
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
}
