package main

import (
	"log"

	"github.com/pluralsh/deployment-operator/internal/mcpserver"
)

func main() {
	server, err := mcpserver.NewMCPServer()
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
}
