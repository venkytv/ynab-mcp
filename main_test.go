package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/tools"
	"github.com/venky/ynab-mcp/ynab"
)

func TestServerToolsList(t *testing.T) {
	client := ynab.NewClient("fake-token", "last-used")

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ynab-mcp",
		Version: "0.1.0",
	}, nil)
	tools.RegisterAll(server, client)

	ct, st := mcp.NewInMemoryTransports()

	ctx := context.Background()
	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := mcpClient.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
		t.Logf("Tool: %s - %s", tool.Name, tool.Description)
	}

	expected := []string{
		"list_budgets", "get_budget",
		"list_accounts", "list_categories", "list_payees",
		"list_transactions", "get_transaction", "create_transaction", "update_transaction",
		"get_month_summary",
	}
	for _, name := range expected {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
	if len(result.Tools) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(result.Tools))
	}
}
