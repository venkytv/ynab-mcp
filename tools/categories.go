package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListCategoriesInput struct {
	BudgetID string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
}

func registerCategoryTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_categories",
		Description: "List all category groups and their categories with budgeted/activity/balance amounts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListCategoriesInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		groups, err := client.ListCategories(ctx, bid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(groups) == 0 {
			return textResult("No categories found."), nil, nil
		}
		var sb strings.Builder
		for _, g := range groups {
			if g.Deleted || g.Hidden {
				continue
			}
			fmt.Fprintf(&sb, "## %s\n", g.Name)
			for _, c := range g.Categories {
				if c.Deleted || c.Hidden {
					continue
				}
				fmt.Fprintf(&sb, "  - %s — Budgeted: %s, Activity: %s, Balance: %s\n",
					c.Name,
					ynab.FormatAmount(c.Budgeted, cf),
					ynab.FormatAmount(c.Activity, cf),
					ynab.FormatAmount(c.Balance, cf))
				fmt.Fprintf(&sb, "    ID: %s\n", c.ID)
			}
		}
		return textResult(sb.String()), nil, nil
	})
}
