package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type GetMonthSummaryInput struct {
	BudgetID string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	Month    string `json:"month" jsonschema:"month in YYYY-MM-DD format using the first of the month (e.g. 2024-03-01), or 'current' for the current month"`
}

func registerMonthTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_month_summary",
		Description: "Get a month's budget summary including income, budgeted amounts, activity, to-be-budgeted, and per-category breakdown.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetMonthSummaryInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		month, err := client.GetMonth(ctx, bid, input.Month)
		if err != nil {
			return errorResult(err), nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Month: %s\n", month.Month)
		fmt.Fprintf(&sb, "Income: %s\n", ynab.FormatAmount(month.Income, cf))
		fmt.Fprintf(&sb, "Budgeted: %s\n", ynab.FormatAmount(month.Budgeted, cf))
		fmt.Fprintf(&sb, "Activity: %s\n", ynab.FormatAmount(month.Activity, cf))
		fmt.Fprintf(&sb, "To Be Budgeted: %s\n", ynab.FormatAmount(month.ToBeBudgeted, cf))
		if month.AgeOfMoney != nil {
			fmt.Fprintf(&sb, "Age of Money: %d days\n", *month.AgeOfMoney)
		}
		sb.WriteString("\nCategory breakdown:\n")
		for _, c := range month.Categories {
			if c.Deleted || c.Hidden {
				continue
			}
			if c.Budgeted == 0 && c.Activity == 0 && c.Balance == 0 {
				continue
			}
			fmt.Fprintf(&sb, "  - %s — Budgeted: %s, Spent: %s, Balance: %s\n",
				c.Name,
				ynab.FormatAmount(c.Budgeted, cf),
				ynab.FormatAmount(c.Activity, cf),
				ynab.FormatAmount(c.Balance, cf))
		}
		return textResult(sb.String()), nil, nil
	})
}
