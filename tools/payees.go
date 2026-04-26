package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListPayeesInput struct {
	BudgetID string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
}

func registerPayeeTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_payees",
		Description: "List all known payees in a budget.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListPayeesInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		payees, err := client.ListPayees(ctx, bid)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(payees) == 0 {
			return textResult("No payees found."), nil, nil
		}
		var sb strings.Builder
		for _, p := range payees {
			if p.Deleted {
				continue
			}
			if p.TransferAccountID != nil {
				continue
			}
			fmt.Fprintf(&sb, "- %s (ID: %s)\n", p.Name, p.ID)
		}
		return textResult(sb.String()), nil, nil
	})
}
