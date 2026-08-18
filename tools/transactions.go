package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type ListTransactionsInput struct {
	BudgetID   string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	SinceDate  string `json:"since_date" jsonschema:"required lower bound; only return transactions on or after this date (YYYY-MM-DD)"`
	UntilDate  string `json:"until_date,omitempty" jsonschema:"only return transactions on or before this date (YYYY-MM-DD)"`
	Type       string `json:"type,omitempty" jsonschema:"filter: 'uncategorized' or 'unapproved'"`
	AccountID  string `json:"account_id,omitempty" jsonschema:"filter by account ID"`
	CategoryID string `json:"category_id,omitempty" jsonschema:"filter by category ID"`
	PayeeID    string `json:"payee_id,omitempty" jsonschema:"filter by payee ID"`
}

type ListTransactionsOutput struct {
	Query               ListTransactionsQuery `json:"query" jsonschema:"effective bounded query scope"`
	Total               int                   `json:"total" jsonschema:"transactions remaining after defensive filtering"`
	Returned            int                   `json:"returned" jsonschema:"transaction rows included in text, capped at 100"`
	Truncated           bool                  `json:"truncated" jsonschema:"whether the text omits in-scope transaction rows"`
	DiscardedOutOfScope int                   `json:"discarded_out_of_scope" jsonschema:"transactions removed by defensive filtering"`
}

type ListTransactionsQuery struct {
	BudgetID   string  `json:"budget_id"`
	SinceDate  string  `json:"since_date"`
	UntilDate  *string `json:"until_date"`
	Type       *string `json:"type"`
	AccountID  *string `json:"account_id"`
	CategoryID *string `json:"category_id"`
	PayeeID    *string `json:"payee_id"`
}

type GetTransactionInput struct {
	BudgetID      string `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	TransactionID string `json:"transaction_id" jsonschema:"the transaction ID to retrieve"`
}

type CreateTransactionInput struct {
	BudgetID        string                `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	AccountID       string                `json:"account_id" jsonschema:"the account ID for this transaction"`
	Date            string                `json:"date" jsonschema:"transaction date (YYYY-MM-DD)"`
	Amount          float64               `json:"amount" jsonschema:"amount in the budget currency (negative for outflow, positive for inflow, e.g. -10.50)"`
	PayeeID         string                `json:"payee_id,omitempty" jsonschema:"payee ID"`
	PayeeName       string                `json:"payee_name,omitempty" jsonschema:"payee name; creates a new payee if no match found"`
	CategoryID      string                `json:"category_id,omitempty" jsonschema:"category ID; omit for split transactions and set category_id on each subtransaction instead"`
	Memo            string                `json:"memo,omitempty" jsonschema:"transaction memo"`
	Cleared         string                `json:"cleared,omitempty" jsonschema:"cleared status: 'cleared', 'uncleared', or 'reconciled'"`
	Approved        *bool                 `json:"approved,omitempty" jsonschema:"whether the transaction is approved"`
	FlagColor       string                `json:"flag_color,omitempty" jsonschema:"flag color: 'red', 'orange', 'yellow', 'green', 'blue', or 'purple'"`
	SubTransactions []SubTransactionInput `json:"subtransactions,omitempty" jsonschema:"split lines; provide at least two and make their amounts add up to amount"`
}

type UpdateTransactionInput struct {
	BudgetID        string                `json:"budget_id,omitempty" jsonschema:"budget ID; uses configured default if omitted"`
	TransactionID   string                `json:"transaction_id" jsonschema:"the transaction ID to update"`
	CategoryID      *string               `json:"category_id,omitempty" jsonschema:"new category ID; do not provide when setting subtransactions"`
	PayeeID         *string               `json:"payee_id,omitempty" jsonschema:"new payee ID"`
	PayeeName       *string               `json:"payee_name,omitempty" jsonschema:"new payee name"`
	Memo            *string               `json:"memo,omitempty" jsonschema:"new memo"`
	Amount          *float64              `json:"amount,omitempty" jsonschema:"new amount in the budget currency"`
	Date            *string               `json:"date,omitempty" jsonschema:"new date (YYYY-MM-DD)"`
	Cleared         *string               `json:"cleared,omitempty" jsonschema:"new cleared status: 'cleared', 'uncleared', or 'reconciled'"`
	Approved        *bool                 `json:"approved,omitempty" jsonschema:"new approval status"`
	FlagColor       *string               `json:"flag_color,omitempty" jsonschema:"new flag color: 'red', 'orange', 'yellow', 'green', 'blue', or 'purple'"`
	SubTransactions []SubTransactionInput `json:"subtransactions,omitempty" jsonschema:"replacement split lines; provide at least two and make their amounts add up to the transaction amount"`
}

type SubTransactionInput struct {
	ID         string  `json:"id,omitempty" jsonschema:"existing split line ID when replacing an existing split transaction"`
	Amount     float64 `json:"amount" jsonschema:"split line amount in the budget currency"`
	CategoryID string  `json:"category_id" jsonschema:"category ID for this split line"`
	PayeeID    string  `json:"payee_id,omitempty" jsonschema:"optional payee ID for this split line"`
	PayeeName  string  `json:"payee_name,omitempty" jsonschema:"optional payee name for this split line"`
	Memo       string  `json:"memo,omitempty" jsonschema:"optional memo for this split line"`
}

func registerTransactionTools(server *mcp.Server, client *ynab.Client) {
	outputSchema, err := jsonschema.For[ListTransactionsOutput](nil)
	if err != nil {
		panic(fmt.Sprintf("list_transactions output schema: %v", err))
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:         "list_transactions",
		Description:  "List transactions using a required since_date lower bound. This tool does not support unbounded queries. Optional filters include until_date, account_id, category_id, payee_id, and type ('uncategorized' or 'unapproved').",
		OutputSchema: outputSchema,
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListTransactionsInput) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(input.SinceDate) == "" {
			return errorResult(fmt.Errorf("since_date must be a non-empty date (YYYY-MM-DD); no YNAB request was made; retry with a non-empty since_date")), nil, nil
		}

		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)

		opts := &ynab.ListTransactionsOptions{
			SinceDate:  input.SinceDate,
			UntilDate:  input.UntilDate,
			Type:       input.Type,
			AccountID:  input.AccountID,
			CategoryID: input.CategoryID,
			PayeeID:    input.PayeeID,
		}
		txns, err := client.ListTransactions(ctx, bid, opts)
		if err != nil {
			return errorResult(err), nil, nil
		}
		txns, discarded := filterTransactionScope(txns, input)
		output := listTransactionsOutput(bid, input, len(txns), discarded)

		var sb strings.Builder
		writeTransactionQueryScope(&sb, output.Query)
		if len(txns) == 0 {
			sb.WriteString(noTransactionsText(discarded))
			return textResult(sb.String()), output, nil
		}
		const maxDisplay = 100
		for i, t := range txns {
			if i >= maxDisplay {
				fmt.Fprintf(&sb, "\n... and %d more transactions. Use since_date or other filters to narrow results.\n", len(txns)-maxDisplay)
				break
			}
			formatTransaction(&sb, &t, cf)
		}
		writeDiscardedTransactionsNote(&sb, discarded)
		fmt.Fprintf(&sb, "\nTotal: %d transactions\n", len(txns))
		return textResult(sb.String()), output, nil
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
		Description: "Create a new transaction. Amount is in the budget's currency: negative for outflows (spending), positive for inflows (income). To create a split transaction, provide subtransactions with per-line category_id values whose amounts add up to amount.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input CreateTransactionInput) (*mcp.CallToolResult, any, error) {
		bid := resolveBudgetID(input.BudgetID, client)
		cf, _ := getCurrencyFormat(ctx, client, bid)
		amount := ynab.DollarsToMilliunits(input.Amount)
		subtransactions, err := buildSaveSubTransactions(input.SubTransactions, amount)
		if err != nil {
			return errorResult(err), nil, nil
		}
		if len(subtransactions) > 0 && input.CategoryID != "" {
			return errorResult(fmt.Errorf("category_id cannot be set on the parent transaction when subtransactions are provided")), nil, nil
		}

		txn := ynab.SaveTransaction{
			AccountID:       input.AccountID,
			Date:            input.Date,
			Amount:          amount,
			PayeeID:         strPtr(input.PayeeID),
			PayeeName:       strPtr(input.PayeeName),
			CategoryID:      strPtr(input.CategoryID),
			Memo:            strPtr(input.Memo),
			Cleared:         strPtr(input.Cleared),
			Approved:        input.Approved,
			FlagColor:       strPtr(input.FlagColor),
			SubTransactions: subtransactions,
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
		Description: "Update an existing transaction. Use this to categorize transactions, change payees, edit memos, etc. Only provided fields are changed. To convert a transaction to a split, provide subtransactions with per-line category_id values whose amounts add up to the transaction amount.",
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
		if len(input.SubTransactions) > 0 {
			if input.CategoryID != nil {
				return errorResult(fmt.Errorf("category_id cannot be set on the parent transaction when subtransactions are provided")), nil, nil
			}
			subtransactions, err := buildSaveSubTransactions(input.SubTransactions, txn.Amount)
			if err != nil {
				return errorResult(err), nil, nil
			}
			txn.CategoryID = nil
			txn.SubTransactions = subtransactions
		} else if len(existing.SubTransactions) > 0 {
			txn.CategoryID = nil
			txn.SubTransactions = existingSaveSubTransactions(existing.SubTransactions)
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

func listTransactionsOutput(budgetID string, input ListTransactionsInput, total, discarded int) ListTransactionsOutput {
	const maxDisplay = 100
	returned := min(total, maxDisplay)
	return ListTransactionsOutput{
		Query: ListTransactionsQuery{
			BudgetID:   budgetID,
			SinceDate:  input.SinceDate,
			UntilDate:  optionalString(input.UntilDate),
			Type:       optionalString(input.Type),
			AccountID:  optionalString(input.AccountID),
			CategoryID: optionalString(input.CategoryID),
			PayeeID:    optionalString(input.PayeeID),
		},
		Total:               total,
		Returned:            returned,
		Truncated:           total > returned,
		DiscardedOutOfScope: discarded,
	}
}

func writeTransactionQueryScope(sb *strings.Builder, query ListTransactionsQuery) {
	sb.WriteString("Query scope:\n")
	fmt.Fprintf(sb, "- budget_id: %s\n", query.BudgetID)
	fmt.Fprintf(sb, "- since_date: %s\n", query.SinceDate)
	fmt.Fprintf(sb, "- until_date: %s\n", queryScopeValue(query.UntilDate))
	fmt.Fprintf(sb, "- type: %s\n", queryScopeValue(query.Type))
	fmt.Fprintf(sb, "- account_id: %s\n", queryScopeValue(query.AccountID))
	fmt.Fprintf(sb, "- category_id: %s\n", queryScopeValue(query.CategoryID))
	fmt.Fprintf(sb, "- payee_id: %s\n\n", queryScopeValue(query.PayeeID))
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func queryScopeValue(value *string) string {
	if value == nil {
		return "not set"
	}
	return *value
}

func filterTransactionScope(txns []ynab.TransactionDetail, input ListTransactionsInput) ([]ynab.TransactionDetail, int) {
	filtered := make([]ynab.TransactionDetail, 0, len(txns))
	for _, txn := range txns {
		if input.SinceDate != "" && txn.Date < input.SinceDate {
			continue
		}
		if input.UntilDate != "" && txn.Date > input.UntilDate {
			continue
		}
		if input.AccountID != "" && txn.AccountID != input.AccountID {
			continue
		}
		filtered = append(filtered, txn)
	}
	return filtered, len(txns) - len(filtered)
}

func noTransactionsText(discarded int) string {
	if discarded == 0 {
		return "No transactions found."
	}
	return fmt.Sprintf(
		"No transactions found.\n\nDefensive filter: discarded %d out-of-scope transaction(s) returned by YNAB.",
		discarded,
	)
}

func writeDiscardedTransactionsNote(sb *strings.Builder, discarded int) {
	if discarded == 0 {
		return
	}
	fmt.Fprintf(
		sb,
		"\nDefensive filter: discarded %d out-of-scope transaction(s) returned by YNAB.\n",
		discarded,
	)
}

func buildSaveSubTransactions(inputs []SubTransactionInput, parentAmount int64) ([]ynab.SaveSubTransaction, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) < 2 {
		return nil, fmt.Errorf("split transactions require at least two subtransactions")
	}

	subtransactions := make([]ynab.SaveSubTransaction, 0, len(inputs))
	var sum int64
	for i, input := range inputs {
		if input.CategoryID == "" {
			return nil, fmt.Errorf("subtransactions[%d].category_id is required", i)
		}
		amount := ynab.DollarsToMilliunits(input.Amount)
		sum += amount
		subtransactions = append(subtransactions, ynab.SaveSubTransaction{
			ID:         strPtr(input.ID),
			Amount:     amount,
			PayeeID:    strPtr(input.PayeeID),
			PayeeName:  strPtr(input.PayeeName),
			CategoryID: strPtr(input.CategoryID),
			Memo:       strPtr(input.Memo),
		})
	}
	if sum != parentAmount {
		return nil, fmt.Errorf("subtransaction amounts sum to %s, but transaction amount is %s",
			ynab.FormatAmount(sum, nil), ynab.FormatAmount(parentAmount, nil))
	}
	return subtransactions, nil
}

func existingSaveSubTransactions(inputs []ynab.SubTransaction) []ynab.SaveSubTransaction {
	subtransactions := make([]ynab.SaveSubTransaction, 0, len(inputs))
	for _, input := range inputs {
		if input.Deleted {
			continue
		}
		subtransactions = append(subtransactions, ynab.SaveSubTransaction{
			ID:         strPtr(input.ID),
			Amount:     input.Amount,
			PayeeID:    input.PayeeID,
			PayeeName:  input.PayeeName,
			CategoryID: input.CategoryID,
			Memo:       input.Memo,
		})
	}
	return subtransactions
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
