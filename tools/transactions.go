package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListTransactionsInput struct {
	BudgetID   string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	SinceDate  string `json:"since_date,omitempty" jsonschema:"only return transactions on or after this date (YYYY-MM-DD); recommended to always provide"`
	Type       string `json:"type,omitempty" jsonschema:"filter: 'uncategorized' or 'unapproved'"`
	AccountID  string `json:"account_id,omitempty" jsonschema:"filter by account ID"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"filter by category ID"`
	PayeeID    string `json:"payee_id,omitempty" jsonschema:"filter by payee ID"`
}

type GetTransactionInput struct {
	BudgetID      string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	TransactionID string `json:"transaction_id" jsonschema:"the transaction ID to retrieve"`
}

type CreateTransactionInput struct {
	BudgetID   string  `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	AccountID  string  `json:"account_id" jsonschema:"the account ID for this transaction"`
	Date       string  `json:"date" jsonschema:"transaction date (YYYY-MM-DD)"`
	Amount     float64 `json:"amount" jsonschema:"amount in the budget currency (negative for outflow, positive for inflow, e.g. -10.50)"`
	PayeeID    string  `json:"payee_id,omitempty" jsonschema:"payee ID"`
	PayeeName  string  `json:"payee_name,omitempty" jsonschema:"payee name; creates a new payee if no match found"`
	CategoryID string  `json:"category_id,omitempty" jsonschema:"category ID"`
	Memo       string  `json:"memo,omitempty" jsonschema:"transaction memo"`
	Cleared    string  `json:"cleared,omitempty" jsonschema:"cleared status: 'cleared', 'uncleared', or 'reconciled'"`
	Approved   *bool   `json:"approved,omitempty" jsonschema:"whether the transaction is approved"`
	FlagColor  string  `json:"flag_color,omitempty" jsonschema:"flag color: 'red', 'orange', 'yellow', 'green', 'blue', or 'purple'"`
}

type UpdateTransactionInput struct {
	BudgetID      string   `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	TransactionID string   `json:"transaction_id" jsonschema:"the transaction ID to update"`
	CategoryID    *string  `json:"category_id,omitempty" jsonschema:"new category ID"`
	PayeeID       *string  `json:"payee_id,omitempty" jsonschema:"new payee ID"`
	PayeeName     *string  `json:"payee_name,omitempty" jsonschema:"new payee name"`
	Memo          *string  `json:"memo,omitempty" jsonschema:"new memo"`
	Amount        *float64 `json:"amount,omitempty" jsonschema:"new amount in the budget currency"`
	Date          *string  `json:"date,omitempty" jsonschema:"new date (YYYY-MM-DD)"`
	Cleared       *string  `json:"cleared,omitempty" jsonschema:"new cleared status: 'cleared', 'uncleared', or 'reconciled'"`
	Approved      *bool    `json:"approved,omitempty" jsonschema:"new approval status"`
	FlagColor     *string  `json:"flag_color,omitempty" jsonschema:"new flag color: 'red', 'orange', 'yellow', 'green', 'blue', or 'purple'"`
}

func registerTransactionTools(server *mcp.Server, client *ynab.Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_transactions",
		Description: "List transactions with optional filters. Provide since_date to limit results. Use type='uncategorized' to find transactions needing categorization, or type='unapproved' for pending transactions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListTransactionsInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		opts := &ynab.ListTransactionsOptions{
			SinceDate:  input.SinceDate,
			Type:       input.Type,
			AccountID:  input.AccountID,
			CategoryID: input.CategoryID,
			PayeeID:    input.PayeeID,
		}
		txns, err := client.ListTransactions(ctx, bid, opts)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(txns) == 0 {
			return textResult("No transactions found."), nil, nil
		}
		var sb strings.Builder
		const maxDisplay = 100
		for i, t := range txns {
			if i >= maxDisplay {
				fmt.Fprintf(&sb, "\n... and %d more transactions. Use since_date or other filters to narrow results.\n", len(txns)-maxDisplay)
				break
			}
			formatTransaction(&sb, &t, cf)
		}
		fmt.Fprintf(&sb, "\nTotal: %d transactions\n", len(txns))
		return textResult(sb.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_transaction",
		Description: "Get full details of a single transaction.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input GetTransactionInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		txn, err := client.GetTransaction(ctx, bid, input.TransactionID)
		if err != nil {
			return errorResult(err), nil, nil
		}
		var sb strings.Builder
		formatTransactionDetail(&sb, txn, cf)
		return textResult(sb.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_transaction",
		Description: "Create a new transaction. Amount is in the budget's currency: negative for outflows (spending), positive for inflows (income).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input CreateTransactionInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		txn := ynab.SaveTransaction{
			AccountID:  input.AccountID,
			Date:       input.Date,
			Amount:     ynab.DollarsToMilliunits(input.Amount),
			PayeeID:    strPtr(input.PayeeID),
			PayeeName:  strPtr(input.PayeeName),
			CategoryID: strPtr(input.CategoryID),
			Memo:       strPtr(input.Memo),
			Cleared:    strPtr(input.Cleared),
			Approved:   input.Approved,
			FlagColor:  strPtr(input.FlagColor),
		}
		created, err := client.CreateTransaction(ctx, bid, txn)
		if err != nil {
			return errorResult(err), nil, nil
		}
		var sb strings.Builder
		sb.WriteString("Transaction created:\n\n")
		formatTransactionDetail(&sb, created, cf)
		return textResult(sb.String()), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_transaction",
		Description: "Update an existing transaction. Use this to categorize transactions, change payees, edit memos, etc. Only provided fields are changed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input UpdateTransactionInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		existing, err := client.GetTransaction(ctx, bid, input.TransactionID)
		if err != nil {
			return errorResult(fmt.Errorf("fetching existing transaction: %w", err)), nil, nil
		}

		txn := ynab.SaveTransaction{
			AccountID: existing.AccountID,
			Date:      existing.Date,
			Amount:    existing.Amount,
		}

		if input.CategoryID != nil {
			txn.CategoryID = input.CategoryID
		} else {
			txn.CategoryID = existing.CategoryID
		}
		if input.PayeeID != nil {
			txn.PayeeID = input.PayeeID
		} else {
			txn.PayeeID = existing.PayeeID
		}
		if input.PayeeName != nil {
			txn.PayeeName = input.PayeeName
		}
		if input.Memo != nil {
			txn.Memo = input.Memo
		} else {
			txn.Memo = existing.Memo
		}
		if input.Amount != nil {
			txn.Amount = ynab.DollarsToMilliunits(*input.Amount)
		}
		if input.Date != nil {
			txn.Date = *input.Date
		}
		if input.Cleared != nil {
			txn.Cleared = input.Cleared
		} else {
			txn.Cleared = &existing.Cleared
		}
		if input.Approved != nil {
			txn.Approved = input.Approved
		} else {
			txn.Approved = &existing.Approved
		}
		if input.FlagColor != nil {
			txn.FlagColor = input.FlagColor
		} else {
			txn.FlagColor = existing.FlagColor
		}

		updated, err := client.UpdateTransaction(ctx, bid, input.TransactionID, txn)
		if err != nil {
			return errorResult(err), nil, nil
		}
		var sb strings.Builder
		sb.WriteString("Transaction updated:\n\n")
		formatTransactionDetail(&sb, updated, cf)
		return textResult(sb.String()), nil, nil
	})
}

func formatTransaction(sb *strings.Builder, t *ynab.TransactionDetail, cf *ynab.CurrencyFormat) {
	payee := deref(t.PayeeName)
	if payee == "" {
		payee = "(no payee)"
	}
	category := deref(t.CategoryName)
	if category == "" {
		category = "(uncategorized)"
	}
	memo := deref(t.Memo)
	fmt.Fprintf(sb, "- %s | %s | %s | %s | %s",
		t.Date, payee, category,
		ynab.FormatAmount(t.Amount, cf), t.AccountName)
	if memo != "" {
		fmt.Fprintf(sb, " | %s", memo)
	}
	fmt.Fprintf(sb, "\n  ID: %s\n", t.ID)
}

func formatTransactionDetail(sb *strings.Builder, t *ynab.TransactionDetail, cf *ynab.CurrencyFormat) {
	fmt.Fprintf(sb, "ID: %s\n", t.ID)
	fmt.Fprintf(sb, "Date: %s\n", t.Date)
	fmt.Fprintf(sb, "Amount: %s\n", ynab.FormatAmount(t.Amount, cf))
	fmt.Fprintf(sb, "Account: %s\n", t.AccountName)
	fmt.Fprintf(sb, "Payee: %s\n", deref(t.PayeeName))
	fmt.Fprintf(sb, "Category: %s\n", deref(t.CategoryName))
	fmt.Fprintf(sb, "Memo: %s\n", deref(t.Memo))
	fmt.Fprintf(sb, "Cleared: %s\n", t.Cleared)
	fmt.Fprintf(sb, "Approved: %v\n", t.Approved)
	if t.FlagColor != nil {
		fmt.Fprintf(sb, "Flag: %s\n", *t.FlagColor)
	}
	if len(t.SubTransactions) > 0 {
		sb.WriteString("Sub-transactions:\n")
		for _, st := range t.SubTransactions {
			if st.Deleted {
				continue
			}
			fmt.Fprintf(sb, "  - %s | %s | %s\n",
				deref(st.CategoryName), ynab.FormatAmount(st.Amount, cf), deref(st.Memo))
		}
	}
}
