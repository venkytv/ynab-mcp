package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/internal/config"
	"github.com/venky/ynab-mcp/tools"
	"github.com/venky/ynab-mcp/ynab"
)

func main() {
	config.LoadFile()

	token := os.Getenv("YNAB_API_TOKEN")
	if token == "" {
		fmt.Fprintf(os.Stderr, "YNAB_API_TOKEN environment variable is required\n")
		os.Exit(1)
	}

	budgetID := os.Getenv("YNAB_BUDGET_ID")
	if budgetID == "" {
		budgetID = "last-used"
	}

	client := ynab.NewClient(token, budgetID)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ynab-mcp",
		Version: "0.1.0",
	}, nil)

	tools.RegisterAll(server, client)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
