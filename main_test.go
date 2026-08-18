package main

import (
	"context"
	"strings"
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

	toolsByName := make(map[string]*mcp.Tool)
	for _, tool := range result.Tools {
		toolsByName[tool.Name] = tool
		t.Logf("Tool: %s - %s", tool.Name, tool.Description)
	}

	expected := []string{
		"list_budgets", "get_budget",
		"list_accounts", "list_categories", "list_payees",
		"list_transactions", "get_transaction", "create_transaction", "update_transaction",
		"get_month_summary",
	}
	for _, name := range expected {
		if toolsByName[name] == nil {
			t.Errorf("missing expected tool: %s", name)
		}
	}
	if len(result.Tools) != len(expected) {
		t.Errorf("expected %d tools, got %d", len(expected), len(result.Tools))
	}

	transactionsTool := toolsByName["list_transactions"]
	if transactionsTool == nil {
		return
	}
	if !strings.Contains(transactionsTool.Description, "required since_date") ||
		!strings.Contains(transactionsTool.Description, "does not support unbounded queries") {
		t.Errorf("list_transactions description does not advertise bounded-only behavior: %q", transactionsTool.Description)
	}
	inputSchema, ok := transactionsTool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("input schema type = %T, want map[string]any", transactionsTool.InputSchema)
	}
	properties, ok := inputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("input schema properties type = %T, want map[string]any", inputSchema["properties"])
	}
	if _, ok := properties["allow_unbounded"]; ok {
		t.Error("input schema unexpectedly exposes allow_unbounded")
	}
	if additional, ok := inputSchema["additionalProperties"].(bool); !ok || additional {
		t.Errorf("additionalProperties = %#v, want false", inputSchema["additionalProperties"])
	}
	required, ok := inputSchema["required"].([]any)
	if !ok {
		t.Fatalf("input schema required type = %T, want []any", inputSchema["required"])
	}
	foundSinceDate := false
	for _, field := range required {
		if field == "since_date" {
			foundSinceDate = true
		}
	}
	if !foundSinceDate {
		t.Errorf("input schema required = %#v, want since_date", required)
	}

	outputSchema, ok := transactionsTool.OutputSchema.(map[string]any)
	if !ok {
		t.Fatalf("output schema type = %T, want map[string]any", transactionsTool.OutputSchema)
	}
	outputProperties, ok := outputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("output schema properties type = %T, want map[string]any", outputSchema["properties"])
	}
	for _, field := range []string{"query", "total", "returned", "truncated", "discarded_out_of_scope"} {
		if _, ok := outputProperties[field]; !ok {
			t.Errorf("output schema missing %q", field)
		}
	}
	querySchema, ok := outputProperties["query"].(map[string]any)
	if !ok {
		t.Fatalf("output query schema type = %T, want map[string]any", outputProperties["query"])
	}
	queryProperties, ok := querySchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("output query properties type = %T, want map[string]any", querySchema["properties"])
	}
	for _, field := range []string{"budget_id", "since_date", "until_date", "type", "account_id", "category_id", "payee_id"} {
		if _, ok := queryProperties[field]; !ok {
			t.Errorf("output query schema missing %q", field)
		}
	}
}
