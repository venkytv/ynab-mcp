package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListBudgetsInput struct{}

type GetBudgetInput struct {
	BudgetID string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
}

func registerBudgetTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_budgets",
		Description: "List all YNAB budgets the user has access to.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListBudgetsInput) (*mcp.CallToolResult, any, error) {
		budgets, err := client.ListBudgets(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(budgets) == 0 {
			return textResult("No budgets found."), nil, nil
		}
		var sb strings.Builder
		for _, b := range budgets {
			fmt.Fprintf(&sb, "- %s (ID: %s, last modified: %s)\n", b.Name, b.ID, b.LastModifiedOn)
		}
		return textResult(sb.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_budget",
		Description: "Get summary details for a budget including date range and currency format.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetBudgetInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		budget, err := client.GetBudget(ctx, bid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Budget: %s\n", budget.Name)
		fmt.Fprintf(&sb, "ID: %s\n", budget.ID)
		fmt.Fprintf(&sb, "Months: %s to %s\n", budget.FirstMonth, budget.LastMonth)
		fmt.Fprintf(&sb, "Last modified: %s\n", budget.LastModifiedOn)
		if budget.CurrencyFormat != nil {
			fmt.Fprintf(&sb, "Currency: %s (%s)\n", budget.CurrencyFormat.ISOCode, budget.CurrencyFormat.CurrencySymbol)
		}
		return textResult(sb.String()), nil, nil
	})
}
