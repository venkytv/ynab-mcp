package ynab

type CurrencyFormat struct {
	ISOCode          string `json:"iso_code"`
	ExampleFormat    string `json:"example_format"`
	DecimalDigits    int    `json:"decimal_digits"`
	DecimalSeparator string `json:"decimal_separator"`
	GroupSeparator   string `json:"group_separator"`
	CurrencySymbol   string `json:"currency_symbol"`
	SymbolFirst      bool   `json:"symbol_first"`
	DisplaySymbol    bool   `json:"display_symbol"`
}

type BudgetSummary struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	LastModifiedOn string          `json:"last_modified_on"`
	FirstMonth     string          `json:"first_month"`
	LastMonth      string          `json:"last_month"`
	CurrencyFormat *CurrencyFormat `json:"currency_format"`
}

type BudgetDetail struct {
	Budget   BudgetSummary `json:"budget"`
	Accounts []Account     `json:"accounts"`
}

type Account struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Type             string  `json:"type"`
	OnBudget         bool    `json:"on_budget"`
	Closed           bool    `json:"closed"`
	Note             *string `json:"note"`
	Balance          int64   `json:"balance"`
	ClearedBalance   int64   `json:"cleared_balance"`
	UnclearedBalance int64   `json:"uncleared_balance"`
	Deleted          bool    `json:"deleted"`
}

type CategoryGroup struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hidden     bool       `json:"hidden"`
	Deleted    bool       `json:"deleted"`
	Categories []Category `json:"categories"`
}

type Category struct {
	ID              string  `json:"id"`
	CategoryGroupID string  `json:"category_group_id"`
	Name            string  `json:"name"`
	Hidden          bool    `json:"hidden"`
	Note            *string `json:"note"`
	Budgeted        int64   `json:"budgeted"`
	Activity        int64   `json:"activity"`
	Balance         int64   `json:"balance"`
	GoalType        *string `json:"goal_type"`
	Deleted         bool    `json:"deleted"`
}

type Payee struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	TransferAccountID *string `json:"transfer_account_id"`
	Deleted           bool    `json:"deleted"`
}

type TransactionDetail struct {
	ID                string           `json:"id"`
	Date              string           `json:"date"`
	Amount            int64            `json:"amount"`
	Memo              *string          `json:"memo"`
	Cleared           string           `json:"cleared"`
	Approved          bool             `json:"approved"`
	FlagColor         *string          `json:"flag_color"`
	FlagName          *string          `json:"flag_name"`
	AccountID         string           `json:"account_id"`
	AccountName       string           `json:"account_name"`
	PayeeID           *string          `json:"payee_id"`
	PayeeName         *string          `json:"payee_name"`
	CategoryID        *string          `json:"category_id"`
	CategoryName      *string          `json:"category_name"`
	TransferAccountID *string          `json:"transfer_account_id"`
	Deleted           bool             `json:"deleted"`
	SubTransactions   []SubTransaction `json:"subtransactions"`
}

type SubTransaction struct {
	ID            string  `json:"id"`
	TransactionID string  `json:"transaction_id"`
	Amount        int64   `json:"amount"`
	Memo          *string `json:"memo"`
	PayeeID       *string `json:"payee_id"`
	PayeeName     *string `json:"payee_name"`
	CategoryID    *string `json:"category_id"`
	CategoryName  *string `json:"category_name"`
	Deleted       bool    `json:"deleted"`
}

type SaveTransaction struct {
	AccountID  string  `json:"account_id"`
	Date       string  `json:"date"`
	Amount     int64   `json:"amount"`
	PayeeID    *string `json:"payee_id,omitempty"`
	PayeeName  *string `json:"payee_name,omitempty"`
	CategoryID *string `json:"category_id,omitempty"`
	Memo       *string `json:"memo,omitempty"`
	Cleared    *string `json:"cleared,omitempty"`
	Approved   *bool   `json:"approved,omitempty"`
	FlagColor  *string `json:"flag_color,omitempty"`
}

type SaveTransactionWrapper struct {
	Transaction SaveTransaction `json:"transaction"`
}

type MonthDetail struct {
	Month        string     `json:"month"`
	Income       int64      `json:"income"`
	Budgeted     int64      `json:"budgeted"`
	Activity     int64      `json:"activity"`
	ToBeBudgeted int64      `json:"to_be_budgeted"`
	AgeOfMoney   *int       `json:"age_of_money"`
	Categories   []Category `json:"categories"`
	Deleted      bool       `json:"deleted"`
}

// API response wrapper types — used only for JSON unmarshaling.

type BudgetListData struct {
	Budgets []BudgetSummary `json:"budgets"`
}

type BudgetDetailData struct {
	Budget BudgetSummary `json:"budget"`
}

type AccountListData struct {
	Accounts []Account `json:"accounts"`
}

type CategoryListData struct {
	CategoryGroups []CategoryGroup `json:"category_groups"`
}

type PayeeListData struct {
	Payees []Payee `json:"payees"`
}

type TransactionListData struct {
	Transactions    []TransactionDetail `json:"transactions"`
	ServerKnowledge int64               `json:"server_knowledge"`
}

type TransactionData struct {
	Transaction TransactionDetail `json:"transaction"`
}

type SaveTransactionsResponseData struct {
	Transaction     *TransactionDetail  `json:"transaction"`
	Transactions    []TransactionDetail `json:"transactions"`
	ServerKnowledge int64               `json:"server_knowledge"`
}

type MonthDetailData struct {
	Month MonthDetail `json:"month"`
}

type ListTransactionsOptions struct {
	SinceDate  string
	Type       string
	AccountID  string
	CategoryID string
	PayeeID    string
}
