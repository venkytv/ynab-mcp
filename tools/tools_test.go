package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

type testEnv struct {
	session *mcp.ClientSession
	mux     *http.ServeMux
	// captured request bodies keyed by "METHOD path"
	bodies   map[string][]byte
	bodiesMu sync.Mutex
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Reset the package-level currency cache between tests
	currencyCacheMu.Lock()
	currencyCache = make(map[string]*ynab.CurrencyFormat)
	currencyCacheMu.Unlock()

	env := &testEnv{
		mux:    http.NewServeMux(),
		bodies: make(map[string][]byte),
	}

	// Default handler for GetBudget (needed by getCurrencyFormat)
	env.mux.HandleFunc("/budgets/test-budget", func(w http.ResponseWriter, r *http.Request) {
		env.captureBody(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"budget": map[string]any{
					"id":   "test-budget",
					"name": "Test Budget",
					"currency_format": map[string]any{
						"currency_symbol":   "$",
						"decimal_digits":    2,
						"decimal_separator": ".",
						"group_separator":   ",",
						"symbol_first":      true,
						"display_symbol":    true,
					},
				},
			},
		})
	})

	ts := httptest.NewServer(env.mux)
	t.Cleanup(ts.Close)

	client := ynab.NewClient("test-token", "test-budget", ynab.WithBaseURL(ts.URL))

	server := mcp.NewServer(&mcp.Implementation{Name: "test-ynab", Version: "0.0.1"}, nil)
	RegisterAll(server, client)

	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()

	serverSession, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverSession.Close() })

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientSession, err := mcpClient.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clientSession.Close() })

	env.session = clientSession
	return env
}

func (e *testEnv) captureBody(r *http.Request) {
	if r.Body == nil {
		return
	}
	body, _ := io.ReadAll(r.Body)
	key := r.Method + " " + r.URL.Path
	e.bodiesMu.Lock()
	e.bodies[key] = body
	e.bodiesMu.Unlock()
}

func callTool(t *testing.T, env *testEnv, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := env.session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s) protocol error: %v", name, err)
	}
	return result
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", result.Content[0])
	}
	return tc.Text
}

func TestListBudgetsTool(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"budgets": []map[string]any{
					{"id": "b1", "name": "Personal", "last_modified_on": "2024-03-01"},
					{"id": "b2", "name": "Business", "last_modified_on": "2024-03-02"},
				},
			},
		})
	})

	result := callTool(t, env, "list_budgets", nil)
	text := toolText(t, result)

	if !strings.Contains(text, "Personal") {
		t.Errorf("missing 'Personal' in output: %s", text)
	}
	if !strings.Contains(text, "Business") {
		t.Errorf("missing 'Business' in output: %s", text)
	}
	if result.IsError {
		t.Errorf("unexpected IsError=true")
	}
}

func TestListTransactionsTool(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets/test-budget/transactions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		payee := "Coffee Shop"
		cat := "Dining Out"
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"transactions": []map[string]any{
					{
						"id": "t1", "date": "2024-03-15", "amount": -4500,
						"account_id": "a1", "account_name": "Checking",
						"payee_name": payee, "category_name": cat,
						"cleared": "cleared", "approved": true,
					},
				},
			},
		})
	})

	result := callTool(t, env, "list_transactions", map[string]any{
		"since_date": "2024-03-01",
	})
	text := toolText(t, result)

	if !strings.Contains(text, "Coffee Shop") {
		t.Errorf("missing payee in output: %s", text)
	}
	if !strings.Contains(text, "$4.50") || !strings.Contains(text, "-") {
		t.Errorf("missing formatted amount in output: %s", text)
	}
	if !strings.Contains(text, "2024-03-15") {
		t.Errorf("missing date in output: %s", text)
	}
}

func TestListTransactionsTool_Uncategorized(t *testing.T) {
	env := setupTestEnv(t)
	var gotType string
	env.mux.HandleFunc("/budgets/test-budget/transactions", func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"transactions": []any{}},
		})
	})

	callTool(t, env, "list_transactions", map[string]any{
		"type": "uncategorized",
	})

	if gotType != "uncategorized" {
		t.Errorf("type query param = %q, want uncategorized", gotType)
	}
}

func TestCreateTransactionTool(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets/test-budget/transactions", func(w http.ResponseWriter, r *http.Request) {
		env.captureBody(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"transaction": map[string]any{
					"id": "new-1", "date": "2024-03-15", "amount": -10500,
					"account_id": "a1", "account_name": "Checking",
					"payee_name": "Grocery Store",
					"cleared":    "uncleared", "approved": false,
				},
			},
		})
	})

	result := callTool(t, env, "create_transaction", map[string]any{
		"account_id": "a1",
		"date":       "2024-03-15",
		"amount":     -10.50,
		"payee_name": "Grocery Store",
	})
	text := toolText(t, result)

	if !strings.Contains(text, "new-1") {
		t.Errorf("missing transaction ID in output: %s", text)
	}

	// Verify milliunits in request body
	env.bodiesMu.Lock()
	body := env.bodies["POST /budgets/test-budget/transactions"]
	env.bodiesMu.Unlock()

	var wrapper struct {
		Transaction struct {
			Amount int64 `json:"amount"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Transaction.Amount != -10500 {
		t.Errorf("request amount = %d milliunits, want -10500", wrapper.Transaction.Amount)
	}
}

func TestCreateSplitTransactionTool(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets/test-budget/transactions", func(w http.ResponseWriter, r *http.Request) {
		env.captureBody(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"transaction": map[string]any{
					"id": "new-1", "date": "2024-03-15", "amount": -10500,
					"account_id": "a1", "account_name": "Checking",
					"payee_name": "Grocery Store",
					"cleared":    "uncleared", "approved": false,
					"subtransactions": []map[string]any{
						{
							"id": "sub-1", "amount": -7000,
							"category_id": "cat-food", "category_name": "Groceries",
							"memo": "food",
						},
						{
							"id": "sub-2", "amount": -3500,
							"category_id": "cat-house", "category_name": "Household",
							"memo": "supplies",
						},
					},
				},
			},
		})
	})

	result := callTool(t, env, "create_transaction", map[string]any{
		"account_id": "a1",
		"date":       "2024-03-15",
		"amount":     -10.50,
		"payee_name": "Grocery Store",
		"subtransactions": []map[string]any{
			{"amount": -7.00, "category_id": "cat-food", "memo": "food"},
			{"amount": -3.50, "category_id": "cat-house", "memo": "supplies"},
		},
	})
	text := toolText(t, result)

	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Sub-transactions:") {
		t.Errorf("missing split details in output: %s", text)
	}

	env.bodiesMu.Lock()
	body := env.bodies["POST /budgets/test-budget/transactions"]
	env.bodiesMu.Unlock()

	var wrapper struct {
		Transaction struct {
			CategoryID      *string `json:"category_id"`
			SubTransactions []struct {
				Amount     int64   `json:"amount"`
				CategoryID *string `json:"category_id"`
				Memo       *string `json:"memo"`
			} `json:"subtransactions"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Transaction.CategoryID != nil {
		t.Errorf("parent category_id = %v, want nil", wrapper.Transaction.CategoryID)
	}
	if len(wrapper.Transaction.SubTransactions) != 2 {
		t.Fatalf("got %d split lines, want 2", len(wrapper.Transaction.SubTransactions))
	}
	if wrapper.Transaction.SubTransactions[0].Amount != -7000 {
		t.Errorf("first split amount = %d, want -7000", wrapper.Transaction.SubTransactions[0].Amount)
	}
	if wrapper.Transaction.SubTransactions[1].CategoryID == nil || *wrapper.Transaction.SubTransactions[1].CategoryID != "cat-house" {
		t.Errorf("second split category_id = %v, want cat-house", wrapper.Transaction.SubTransactions[1].CategoryID)
	}
}

func TestCreateSplitTransactionTool_AmountMismatch(t *testing.T) {
	env := setupTestEnv(t)

	result := callTool(t, env, "create_transaction", map[string]any{
		"account_id": "a1",
		"date":       "2024-03-15",
		"amount":     -10.50,
		"subtransactions": []map[string]any{
			{"amount": -7.00, "category_id": "cat-food"},
			{"amount": -2.00, "category_id": "cat-house"},
		},
	})
	text := toolText(t, result)

	if !result.IsError {
		t.Fatalf("expected error, got: %s", text)
	}
	if !strings.Contains(text, "subtransaction amounts sum") {
		t.Errorf("unexpected error text: %s", text)
	}
}

func TestUpdateTransactionTool_PartialUpdate(t *testing.T) {
	env := setupTestEnv(t)

	// GET existing transaction
	env.mux.HandleFunc("/budgets/test-budget/transactions/txn-1", func(w http.ResponseWriter, r *http.Request) {
		env.captureBody(r)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			memo := "original memo"
			payeeID := "p1"
			catID := "old-cat"
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"transaction": map[string]any{
						"id": "txn-1", "date": "2024-03-15", "amount": -5000,
						"account_id": "a1", "account_name": "Checking",
						"payee_id": payeeID, "payee_name": "Old Payee",
						"category_id": catID, "memo": memo,
						"cleared": "uncleared", "approved": false,
					},
				},
			})
		} else {
			// PUT response
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"transaction": map[string]any{
						"id": "txn-1", "date": "2024-03-15", "amount": -5000,
						"account_id": "a1", "account_name": "Checking",
						"category_id": "new-cat", "category_name": "Groceries",
						"memo":    "original memo",
						"cleared": "uncleared", "approved": false,
					},
				},
			})
		}
	})

	result := callTool(t, env, "update_transaction", map[string]any{
		"transaction_id": "txn-1",
		"category_id":    "new-cat",
	})
	text := toolText(t, result)

	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "txn-1") {
		t.Errorf("missing transaction ID: %s", text)
	}

	// Verify the PUT body preserves unchanged fields
	env.bodiesMu.Lock()
	body := env.bodies["PUT /budgets/test-budget/transactions/txn-1"]
	env.bodiesMu.Unlock()

	var wrapper struct {
		Transaction struct {
			AccountID  string  `json:"account_id"`
			Amount     int64   `json:"amount"`
			CategoryID *string `json:"category_id"`
			Memo       *string `json:"memo"`
			Cleared    *string `json:"cleared"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Transaction.AccountID != "a1" {
		t.Errorf("preserved account_id = %q, want a1", wrapper.Transaction.AccountID)
	}
	if wrapper.Transaction.Amount != -5000 {
		t.Errorf("preserved amount = %d, want -5000", wrapper.Transaction.Amount)
	}
	if wrapper.Transaction.CategoryID == nil || *wrapper.Transaction.CategoryID != "new-cat" {
		t.Errorf("category_id = %v, want new-cat", wrapper.Transaction.CategoryID)
	}
	if wrapper.Transaction.Memo == nil || *wrapper.Transaction.Memo != "original memo" {
		t.Errorf("memo = %v, want 'original memo'", wrapper.Transaction.Memo)
	}
}

func TestUpdateTransactionTool_ToSplit(t *testing.T) {
	env := setupTestEnv(t)

	env.mux.HandleFunc("/budgets/test-budget/transactions/txn-1", func(w http.ResponseWriter, r *http.Request) {
		env.captureBody(r)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			memo := "original memo"
			payeeID := "p1"
			catID := "old-cat"
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"transaction": map[string]any{
						"id": "txn-1", "date": "2024-03-15", "amount": -5000,
						"account_id": "a1", "account_name": "Checking",
						"payee_id": payeeID, "payee_name": "Old Payee",
						"category_id": catID, "memo": memo,
						"cleared": "uncleared", "approved": false,
					},
				},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"transaction": map[string]any{
					"id": "txn-1", "date": "2024-03-15", "amount": -5000,
					"account_id": "a1", "account_name": "Checking",
					"memo":    "original memo",
					"cleared": "uncleared", "approved": false,
					"subtransactions": []map[string]any{
						{
							"id": "sub-1", "amount": -3000,
							"category_id": "cat-food", "category_name": "Groceries",
						},
						{
							"id": "sub-2", "amount": -2000,
							"category_id": "cat-house", "category_name": "Household",
						},
					},
				},
			},
		})
	})

	result := callTool(t, env, "update_transaction", map[string]any{
		"transaction_id": "txn-1",
		"subtransactions": []map[string]any{
			{"amount": -3.00, "category_id": "cat-food"},
			{"amount": -2.00, "category_id": "cat-house"},
		},
	})
	text := toolText(t, result)

	if result.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	if !strings.Contains(text, "Sub-transactions:") {
		t.Errorf("missing split details in output: %s", text)
	}

	env.bodiesMu.Lock()
	body := env.bodies["PUT /budgets/test-budget/transactions/txn-1"]
	env.bodiesMu.Unlock()

	var wrapper struct {
		Transaction struct {
			AccountID       string  `json:"account_id"`
			Amount          int64   `json:"amount"`
			CategoryID      *string `json:"category_id"`
			Memo            *string `json:"memo"`
			SubTransactions []struct {
				Amount     int64   `json:"amount"`
				CategoryID *string `json:"category_id"`
			} `json:"subtransactions"`
		} `json:"transaction"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		t.Fatal(err)
	}
	if wrapper.Transaction.AccountID != "a1" {
		t.Errorf("preserved account_id = %q, want a1", wrapper.Transaction.AccountID)
	}
	if wrapper.Transaction.Amount != -5000 {
		t.Errorf("preserved amount = %d, want -5000", wrapper.Transaction.Amount)
	}
	if wrapper.Transaction.CategoryID != nil {
		t.Errorf("parent category_id = %v, want nil", wrapper.Transaction.CategoryID)
	}
	if wrapper.Transaction.Memo == nil || *wrapper.Transaction.Memo != "original memo" {
		t.Errorf("memo = %v, want original memo", wrapper.Transaction.Memo)
	}
	if len(wrapper.Transaction.SubTransactions) != 2 {
		t.Fatalf("got %d split lines, want 2", len(wrapper.Transaction.SubTransactions))
	}
	if wrapper.Transaction.SubTransactions[0].Amount != -3000 {
		t.Errorf("first split amount = %d, want -3000", wrapper.Transaction.SubTransactions[0].Amount)
	}
}

func TestToolAPIError(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets/test-budget/payees", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"id":     "404",
				"name":   "not_found",
				"detail": "Budget not found",
			},
		})
	})

	result := callTool(t, env, "list_payees", nil)
	if !result.IsError {
		t.Error("expected IsError=true")
	}
	text := toolText(t, result)
	if !strings.Contains(text, "not_found") {
		t.Errorf("error text doesn't contain error name: %s", text)
	}
}

func TestGetMonthSummaryTool(t *testing.T) {
	env := setupTestEnv(t)
	env.mux.HandleFunc("/budgets/test-budget/months/2024-03-01", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"month": map[string]any{
					"month":          "2024-03-01",
					"income":         5000000,
					"budgeted":       4500000,
					"activity":       -3200000,
					"to_be_budgeted": 500000,
					"age_of_money":   45,
					"categories": []map[string]any{
						{"id": "c1", "name": "Rent", "budgeted": 1500000, "activity": -1500000, "balance": 0},
						{"id": "c2", "name": "Groceries", "budgeted": 500000, "activity": -320000, "balance": 180000},
					},
				},
			},
		})
	})

	result := callTool(t, env, "get_month_summary", map[string]any{
		"month": "2024-03-01",
	})
	text := toolText(t, result)

	if !strings.Contains(text, "$5,000.00") {
		t.Errorf("missing formatted income: %s", text)
	}
	if !strings.Contains(text, "45 days") {
		t.Errorf("missing age of money: %s", text)
	}
	if !strings.Contains(text, "Rent") {
		t.Errorf("missing category breakdown: %s", text)
	}
	if !strings.Contains(text, "Groceries") {
		t.Errorf("missing category: %s", text)
	}
}

func TestBudgetIDFallback(t *testing.T) {
	env := setupTestEnv(t)
	var gotPath string
	env.mux.HandleFunc("/budgets/test-budget/payees", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"payees": []any{}},
		})
	})

	// Call without budget_id — should use client default "test-budget"
	callTool(t, env, "list_payees", nil)

	if gotPath != "/budgets/test-budget/payees" {
		t.Errorf("path = %q, want /budgets/test-budget/payees", gotPath)
	}
}
