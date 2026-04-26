package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListAccountsInput struct {
	BudgetID string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
}

func registerAccountTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_accounts",
		Description: "List all accounts in a budget with balances.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListAccountsInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		accounts, err := client.ListAccounts(ctx, bid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(accounts) == 0 {
			return textResult("No accounts found."), nil, nil
		}
		var sb strings.Builder
		for _, a := range accounts {
			if a.Deleted {
				continue
			}
			status := "open"
			if a.Closed {
				status = "closed"
			}
			budgetStatus := "on-budget"
			if !a.OnBudget {
				budgetStatus = "tracking"
			}
			fmt.Fprintf(&sb, "- %s (%s, %s, %s) — Balance: %s\n",
				a.Name, a.Type, budgetStatus, status,
				ynab.FormatAmount(a.Balance, cf))
			fmt.Fprintf(&sb, "  ID: %s\n", a.ID)
		}
		return textResult(sb.String()), nil, nil
	})
}
