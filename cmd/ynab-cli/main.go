package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/venky/ynab-mcp/internal/config"
	"github.com/venky/ynab-mcp/ynab"
)

const usage = `Usage: ynab-cli <command> [options]
       ynab-cli --version

Read-only commands:
  list-budgets
  get-budget
  list-accounts
  list-categories
  list-payees
  list-transactions
  get-transaction
  get-month-summary

Run "ynab-cli <command> -h" for command options.
`

var version = "0.2.0"

type outputFormat string

const (
	formatText    outputFormat = "text"
	formatRawJSON outputFormat = "raw-json"
)

func main() {
	os.Exit(realMain(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func realMain(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if args[0] == "--version" {
		if len(args) != 1 {
			fmt.Fprintln(stderr, "ynab-cli: --version does not accept arguments")
			return 2
		}
		fmt.Fprintf(stdout, "{\"name\":\"ynab-cli\",\"version\":%q}\n", version)
		return 0
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) == 2 && (args[1] == "-h" || args[1] == "--help") {
		client := ynab.NewClient("", "last-used")
		if err := run(ctx, client, args, stdout, stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintf(stderr, "ynab-cli: %v\n", err)
			return 1
		}
		return 0
	}

	config.LoadFile()

	token := os.Getenv("YNAB_API_TOKEN")
	if token == "" {
		fmt.Fprintln(stderr, "ynab-cli: YNAB_API_TOKEN environment variable is required")
		return 1
	}

	budgetID := os.Getenv("YNAB_BUDGET_ID")
	if budgetID == "" {
		budgetID = "last-used"
	}

	client := ynab.NewClient(token, budgetID)
	return runAndReport(ctx, client, args, stdout, stderr)
}

func runAndReport(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) int {
	if err := run(ctx, client, args, stdout, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(stderr, "ynab-cli: %v\n", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	switch args[0] {
	case "list-budgets":
		return listBudgets(ctx, client, args[1:], stdout, stderr)
	case "get-budget":
		return getBudget(ctx, client, args[1:], stdout, stderr)
	case "list-accounts":
		return listAccounts(ctx, client, args[1:], stdout, stderr)
	case "list-categories":
		return listCategories(ctx, client, args[1:], stdout, stderr)
	case "list-payees":
		return listPayees(ctx, client, args[1:], stdout, stderr)
	case "list-transactions":
		return listTransactions(ctx, client, args[1:], stdout, stderr)
	case "get-transaction":
		return getTransaction(ctx, client, args[1:], stdout, stderr)
	case "get-month-summary":
		return getMonthSummary(ctx, client, args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func addBudgetID(fs *flag.FlagSet) *string {
	return fs.String("budget-id", "", "budget ID (defaults to YNAB_BUDGET_ID)")
}

func addFormat(fs *flag.FlagSet) *string {
	return fs.String("format", string(formatText), "output format: text or raw-json")
}

func parseReadOptions(
	fs *flag.FlagSet,
	args []string,
	client *ynab.Client,
	budgetID *string,
	formatValue *string,
) (string, outputFormat, error) {
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if fs.NArg() != 0 {
		return "", "", fmt.Errorf("%s: unexpected arguments: %s", fs.Name(), strings.Join(fs.Args(), " "))
	}
	format := outputFormat(*formatValue)
	if format != formatText && format != formatRawJSON {
		return "", "", fmt.Errorf("%s: format must be text or raw-json", fs.Name())
	}
	if budgetID == nil {
		return "", format, nil
	}
	if *budgetID == "" {
		*budgetID = client.BudgetID()
	}
	return *budgetID, format, nil
}

func writeRawResponse(stdout io.Writer, response []byte) error {
	if _, err := stdout.Write(response); err != nil {
		return fmt.Errorf("writing raw JSON: %w", err)
	}
	return nil
}

func currencyFormat(ctx context.Context, client *ynab.Client, budgetID string) (*ynab.CurrencyFormat, error) {
	budget, err := client.GetBudget(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("get budget currency: %w", err)
	}
	return budget.CurrencyFormat, nil
}

func listBudgets(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list-budgets", stderr)
	formatValue := addFormat(fs)
	_, format, err := parseReadOptions(fs, args, client, nil, formatValue)
	if err != nil {
		return err
	}
	if format == formatRawJSON {
		response, err := client.RawListBudgets(ctx)
		if err != nil {
			return fmt.Errorf("list budgets: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	budgets, err := client.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}
	if len(budgets) == 0 {
		fmt.Fprintln(stdout, "No budgets found.")
		return nil
	}
	for _, budget := range budgets {
		fmt.Fprintf(stdout, "- %s (ID: %s, last modified: %s)\n",
			budget.Name, budget.ID, budget.LastModifiedOn)
	}
	return nil
}

func getBudget(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("get-budget", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if format == formatRawJSON {
		response, err := client.RawGetBudget(ctx, budgetID)
		if err != nil {
			return fmt.Errorf("get budget: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	budget, err := client.GetBudget(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("get budget: %w", err)
	}
	fmt.Fprintf(stdout, "Budget: %s\n", budget.Name)
	fmt.Fprintf(stdout, "ID: %s\n", budget.ID)
	fmt.Fprintf(stdout, "Months: %s to %s\n", budget.FirstMonth, budget.LastMonth)
	fmt.Fprintf(stdout, "Last modified: %s\n", budget.LastModifiedOn)
	if budget.CurrencyFormat != nil {
		fmt.Fprintf(stdout, "Currency: %s (%s)\n",
			budget.CurrencyFormat.ISOCode, budget.CurrencyFormat.CurrencySymbol)
	}
	return nil
}

func listAccounts(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list-accounts", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if format == formatRawJSON {
		response, err := client.RawListAccounts(ctx, budgetID)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		return writeRawResponse(stdout, response)
	}
	cf, err := currencyFormat(ctx, client, budgetID)
	if err != nil {
		return err
	}

	accounts, err := client.ListAccounts(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	count := 0
	for _, account := range accounts {
		if account.Deleted {
			continue
		}
		count++
		status := "open"
		if account.Closed {
			status = "closed"
		}
		budgetStatus := "on-budget"
		if !account.OnBudget {
			budgetStatus = "tracking"
		}
		fmt.Fprintf(stdout, "- %s (%s, %s, %s) — Balance: %s\n",
			account.Name, account.Type, budgetStatus, status,
			ynab.FormatAmount(account.Balance, cf))
		fmt.Fprintf(stdout, "  ID: %s\n", account.ID)
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No accounts found.")
	}
	return nil
}

func listCategories(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list-categories", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if format == formatRawJSON {
		response, err := client.RawListCategories(ctx, budgetID)
		if err != nil {
			return fmt.Errorf("list categories: %w", err)
		}
		return writeRawResponse(stdout, response)
	}
	cf, err := currencyFormat(ctx, client, budgetID)
	if err != nil {
		return err
	}

	groups, err := client.ListCategories(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}
	count := 0
	for _, group := range groups {
		if group.Deleted || group.Hidden {
			continue
		}
		var visible []ynab.Category
		for _, category := range group.Categories {
			if !category.Deleted && !category.Hidden {
				visible = append(visible, category)
			}
		}
		if len(visible) == 0 {
			continue
		}
		fmt.Fprintf(stdout, "## %s\n", group.Name)
		for _, category := range visible {
			count++
			fmt.Fprintf(stdout, "  - %s — Budgeted: %s, Activity: %s, Balance: %s\n",
				category.Name,
				ynab.FormatAmount(category.Budgeted, cf),
				ynab.FormatAmount(category.Activity, cf),
				ynab.FormatAmount(category.Balance, cf))
			fmt.Fprintf(stdout, "    ID: %s\n", category.ID)
		}
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No categories found.")
	}
	return nil
}

func listPayees(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list-payees", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if format == formatRawJSON {
		response, err := client.RawListPayees(ctx, budgetID)
		if err != nil {
			return fmt.Errorf("list payees: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	payees, err := client.ListPayees(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("list payees: %w", err)
	}
	count := 0
	for _, payee := range payees {
		if payee.Deleted || payee.TransferAccountID != nil {
			continue
		}
		count++
		fmt.Fprintf(stdout, "- %s (ID: %s)\n", payee.Name, payee.ID)
	}
	if count == 0 {
		fmt.Fprintln(stdout, "No payees found.")
	}
	return nil
}

func listTransactions(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("list-transactions", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	sinceDate := fs.String("since-date", "", "only transactions on or after YYYY-MM-DD")
	untilDate := fs.String("until-date", "", "only transactions on or before YYYY-MM-DD")
	transactionType := fs.String("type", "", "uncategorized or unapproved")
	accountID := fs.String("account-id", "", "filter by account ID")
	categoryID := fs.String("category-id", "", "filter by category ID")
	payeeID := fs.String("payee-id", "", "filter by payee ID")
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if *transactionType != "" && *transactionType != "uncategorized" && *transactionType != "unapproved" {
		return fmt.Errorf("list-transactions: type must be uncategorized or unapproved")
	}
	filterCount := 0
	for _, value := range []string{*accountID, *categoryID, *payeeID} {
		if value != "" {
			filterCount++
		}
	}
	if filterCount > 1 {
		return fmt.Errorf("list-transactions: account-id, category-id, and payee-id are mutually exclusive")
	}

	options := &ynab.ListTransactionsOptions{
		SinceDate:  *sinceDate,
		UntilDate:  *untilDate,
		Type:       *transactionType,
		AccountID:  *accountID,
		CategoryID: *categoryID,
		PayeeID:    *payeeID,
	}
	if format == formatRawJSON {
		response, err := client.RawListTransactions(ctx, budgetID, options)
		if err != nil {
			return fmt.Errorf("list transactions: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	cf, err := currencyFormat(ctx, client, budgetID)
	if err != nil {
		return err
	}
	transactions, err := client.ListTransactions(ctx, budgetID, options)
	if err != nil {
		return fmt.Errorf("list transactions: %w", err)
	}
	if len(transactions) == 0 {
		fmt.Fprintln(stdout, "No transactions found.")
		return nil
	}
	for i := range transactions {
		formatTransaction(stdout, &transactions[i], cf)
	}
	fmt.Fprintf(stdout, "\nTotal: %d transactions\n", len(transactions))
	return nil
}

func getTransaction(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("get-transaction", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	transactionID := fs.String("transaction-id", "", "transaction ID (required)")
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if *transactionID == "" {
		return fmt.Errorf("get-transaction: transaction-id is required")
	}
	if format == formatRawJSON {
		response, err := client.RawGetTransaction(ctx, budgetID, *transactionID)
		if err != nil {
			return fmt.Errorf("get transaction: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	cf, err := currencyFormat(ctx, client, budgetID)
	if err != nil {
		return err
	}
	transaction, err := client.GetTransaction(ctx, budgetID, *transactionID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}
	formatTransactionDetail(stdout, transaction, cf)
	return nil
}

func getMonthSummary(ctx context.Context, client *ynab.Client, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("get-month-summary", stderr)
	budgetIDValue := addBudgetID(fs)
	formatValue := addFormat(fs)
	monthName := fs.String("month", "", "month as YYYY-MM-01 or current (required)")
	budgetID, format, err := parseReadOptions(fs, args, client, budgetIDValue, formatValue)
	if err != nil {
		return err
	}
	if *monthName == "" {
		return fmt.Errorf("get-month-summary: month is required")
	}
	if format == formatRawJSON {
		response, err := client.RawGetMonth(ctx, budgetID, *monthName)
		if err != nil {
			return fmt.Errorf("get month summary: %w", err)
		}
		return writeRawResponse(stdout, response)
	}

	cf, err := currencyFormat(ctx, client, budgetID)
	if err != nil {
		return err
	}
	month, err := client.GetMonth(ctx, budgetID, *monthName)
	if err != nil {
		return fmt.Errorf("get month summary: %w", err)
	}
	fmt.Fprintf(stdout, "Month: %s\n", month.Month)
	fmt.Fprintf(stdout, "Income: %s\n", ynab.FormatAmount(month.Income, cf))
	fmt.Fprintf(stdout, "Budgeted: %s\n", ynab.FormatAmount(month.Budgeted, cf))
	fmt.Fprintf(stdout, "Activity: %s\n", ynab.FormatAmount(month.Activity, cf))
	fmt.Fprintf(stdout, "To Be Budgeted: %s\n", ynab.FormatAmount(month.ToBeBudgeted, cf))
	if month.AgeOfMoney != nil {
		fmt.Fprintf(stdout, "Age of Money: %d days\n", *month.AgeOfMoney)
	}
	fmt.Fprintln(stdout, "\nCategory breakdown:")
	for _, category := range month.Categories {
		if category.Deleted || category.Hidden {
			continue
		}
		if category.Budgeted == 0 && category.Activity == 0 && category.Balance == 0 {
			continue
		}
		fmt.Fprintf(stdout, "  - %s — Budgeted: %s, Spent: %s, Balance: %s\n",
			category.Name,
			ynab.FormatAmount(category.Budgeted, cf),
			ynab.FormatAmount(category.Activity, cf),
			ynab.FormatAmount(category.Balance, cf))
	}
	return nil
}

func formatTransaction(w io.Writer, transaction *ynab.TransactionDetail, cf *ynab.CurrencyFormat) {
	payee := deref(transaction.PayeeName)
	if payee == "" {
		payee = "(no payee)"
	}
	category := deref(transaction.CategoryName)
	if category == "" {
		category = "(uncategorized)"
	}
	fmt.Fprintf(w, "- %s | %s | %s | %s | %s",
		transaction.Date, payee, category,
		ynab.FormatAmount(transaction.Amount, cf), transaction.AccountName)
	if memo := deref(transaction.Memo); memo != "" {
		fmt.Fprintf(w, " | %s", memo)
	}
	fmt.Fprintf(w, "\n  ID: %s\n", transaction.ID)
}

func formatTransactionDetail(w io.Writer, transaction *ynab.TransactionDetail, cf *ynab.CurrencyFormat) {
	fmt.Fprintf(w, "ID: %s\n", transaction.ID)
	fmt.Fprintf(w, "Date: %s\n", transaction.Date)
	fmt.Fprintf(w, "Amount: %s\n", ynab.FormatAmount(transaction.Amount, cf))
	fmt.Fprintf(w, "Account: %s\n", transaction.AccountName)
	fmt.Fprintf(w, "Payee: %s\n", deref(transaction.PayeeName))
	fmt.Fprintf(w, "Category: %s\n", deref(transaction.CategoryName))
	fmt.Fprintf(w, "Memo: %s\n", deref(transaction.Memo))
	fmt.Fprintf(w, "Cleared: %s\n", transaction.Cleared)
	fmt.Fprintf(w, "Approved: %v\n", transaction.Approved)
	if transaction.FlagColor != nil {
		fmt.Fprintf(w, "Flag: %s\n", *transaction.FlagColor)
	}
	if len(transaction.SubTransactions) == 0 {
		return
	}
	fmt.Fprintln(w, "Sub-transactions:")
	for _, subtransaction := range transaction.SubTransactions {
		if subtransaction.Deleted {
			continue
		}
		fmt.Fprintf(w, "  - %s | %s | %s\n",
			deref(subtransaction.CategoryName),
			ynab.FormatAmount(subtransaction.Amount, cf),
			deref(subtransaction.Memo))
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
