package ynab

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	client := NewClient("test-token", "budget-1", WithBaseURL(ts.URL))
	return client, ts
}

func jsonResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		jsonResponse(t, w, map[string]any{"budgets": []any{}})
	})

	client.ListBudgets(context.Background())
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
}

func TestListBudgets(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets" {
			t.Errorf("path = %q, want /budgets", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		jsonResponse(t, w, map[string]any{
			"budgets": []map[string]any{
				{"id": "b1", "name": "My Budget"},
				{"id": "b2", "name": "Other Budget"},
			},
		})
	})

	budgets, err := client.ListBudgets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(budgets) != 2 {
		t.Fatalf("got %d budgets, want 2", len(budgets))
	}
	if budgets[0].ID != "b1" || budgets[0].Name != "My Budget" {
		t.Errorf("budget[0] = %+v", budgets[0])
	}
}

func TestGetBudget(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"budget": map[string]any{
				"id":   "budget-1",
				"name": "Test Budget",
				"currency_format": map[string]any{
					"currency_symbol":   "€",
					"decimal_digits":    2,
					"decimal_separator": ",",
					"group_separator":   ".",
					"symbol_first":      false,
					"display_symbol":    true,
				},
			},
		})
	})

	budget, err := client.GetBudget(context.Background(), "budget-1")
	if err != nil {
		t.Fatal(err)
	}
	if budget.Name != "Test Budget" {
		t.Errorf("name = %q", budget.Name)
	}
	if budget.CurrencyFormat == nil {
		t.Fatal("currency_format is nil")
	}
	if budget.CurrencyFormat.CurrencySymbol != "€" {
		t.Errorf("currency symbol = %q", budget.CurrencyFormat.CurrencySymbol)
	}
}

func TestListAccounts(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/accounts" {
			t.Errorf("path = %q", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"accounts": []map[string]any{
				{"id": "a1", "name": "Checking", "type": "checking", "balance": 500000, "on_budget": true},
			},
		})
	})

	accounts, err := client.ListAccounts(context.Background(), "budget-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts", len(accounts))
	}
	if accounts[0].Balance != 500000 {
		t.Errorf("balance = %d, want 500000", accounts[0].Balance)
	}
}

func TestListCategories(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/categories" {
			t.Errorf("path = %q", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"category_groups": []map[string]any{
				{
					"id":   "cg1",
					"name": "Bills",
					"categories": []map[string]any{
						{"id": "c1", "name": "Rent", "budgeted": 1500000, "activity": -1500000, "balance": 0},
					},
				},
			},
		})
	})

	groups, err := client.ListCategories(context.Background(), "budget-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("got %d groups", len(groups))
	}
	if len(groups[0].Categories) != 1 {
		t.Fatalf("got %d categories", len(groups[0].Categories))
	}
	if groups[0].Categories[0].Budgeted != 1500000 {
		t.Errorf("budgeted = %d", groups[0].Categories[0].Budgeted)
	}
}

func TestListPayees(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/payees" {
			t.Errorf("path = %q", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"payees": []map[string]any{
				{"id": "p1", "name": "Grocery Store"},
				{"id": "p2", "name": "Electric Company"},
			},
		})
	})

	payees, err := client.ListPayees(context.Background(), "budget-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(payees) != 2 {
		t.Fatalf("got %d payees", len(payees))
	}
}

func TestListTransactions_Default(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/transactions" {
			t.Errorf("path = %q, want /budgets/budget-1/transactions", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"transactions": []map[string]any{
				{"id": "t1", "date": "2024-03-01", "amount": -10500},
			},
		})
	})

	txns, err := client.ListTransactions(context.Background(), "budget-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 1 {
		t.Fatalf("got %d transactions", len(txns))
	}
	if txns[0].Amount != -10500 {
		t.Errorf("amount = %d, want -10500", txns[0].Amount)
	}
}

func TestListTransactions_ByAccount(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		want := "/budgets/budget-1/accounts/acc-1/transactions"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		jsonResponse(t, w, map[string]any{"transactions": []any{}})
	})

	client.ListTransactions(context.Background(), "budget-1", &ListTransactionsOptions{AccountID: "acc-1"})
}

func TestListTransactions_ByCategory(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		want := "/budgets/budget-1/categories/cat-1/transactions"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		jsonResponse(t, w, map[string]any{"transactions": []any{}})
	})

	client.ListTransactions(context.Background(), "budget-1", &ListTransactionsOptions{CategoryID: "cat-1"})
}

func TestListTransactions_ByPayee(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		want := "/budgets/budget-1/payees/pay-1/transactions"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		jsonResponse(t, w, map[string]any{"transactions": []any{}})
	})

	client.ListTransactions(context.Background(), "budget-1", &ListTransactionsOptions{PayeeID: "pay-1"})
}

func TestListTransactions_QueryParams(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("since_date"); got != "2024-01-01" {
			t.Errorf("since_date = %q, want 2024-01-01", got)
		}
		if got := r.URL.Query().Get("until_date"); got != "2024-12-31" {
			t.Errorf("until_date = %q, want 2024-12-31", got)
		}
		if got := r.URL.Query().Get("type"); got != "unapproved" {
			t.Errorf("type = %q, want unapproved", got)
		}
		jsonResponse(t, w, map[string]any{"transactions": []any{}})
	})

	client.ListTransactions(context.Background(), "budget-1", &ListTransactionsOptions{
		SinceDate: "2024-01-01",
		UntilDate: "2024-12-31",
		Type:      "unapproved",
	})
}

func TestRawListTransactionsPreservesResponse(t *testing.T) {
	response := []byte("{\n  \"data\": {\n    \"transactions\": [{\"id\":\"t1\",\"unknown\":null,\"deleted\":true,\"transfer_account_id\":\"a2\",\"subtransactions\":[{\"id\":\"s1\",\"new_amount_field\":\"1.25\"}]}],\n    \"server_knowledge\": 42,\n    \"new_field\": {\"nested\": true}\n  }\n}")
	requests := 0
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/budgets/budget-1/transactions" {
			t.Errorf("path = %q, want /budgets/budget-1/transactions", r.URL.Path)
		}
		if got := r.URL.Query().Get("since_date"); got != "2025-04-06" {
			t.Errorf("since_date = %q, want 2025-04-06", got)
		}
		if got := r.URL.Query().Get("until_date"); got != "2026-04-05" {
			t.Errorf("until_date = %q, want 2026-04-05", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	got, err := client.RawListTransactions(context.Background(), "budget-1", &ListTransactionsOptions{
		SinceDate: "2025-04-06",
		UntilDate: "2026-04-05",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, response) {
		t.Errorf("raw response changed:\ngot:  %q\nwant: %q", got, response)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
}

func TestGetTransaction(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/transactions/txn-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		memo := "lunch"
		jsonResponse(t, w, map[string]any{
			"transaction": map[string]any{
				"id": "txn-1", "date": "2024-03-15", "amount": -8500,
				"account_id": "a1", "account_name": "Checking",
				"memo": memo, "cleared": "cleared", "approved": true,
			},
		})
	})

	txn, err := client.GetTransaction(context.Background(), "budget-1", "txn-1")
	if err != nil {
		t.Fatal(err)
	}
	if txn.ID != "txn-1" {
		t.Errorf("id = %q", txn.ID)
	}
	if txn.Amount != -8500 {
		t.Errorf("amount = %d", txn.Amount)
	}
	if txn.Memo == nil || *txn.Memo != "lunch" {
		t.Errorf("memo = %v", txn.Memo)
	}
}

func TestCreateTransaction(t *testing.T) {
	var gotBody SaveTransactionWrapper
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/budgets/budget-1/transactions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		jsonResponse(t, w, map[string]any{
			"transaction": map[string]any{
				"id": "new-1", "date": "2024-03-15", "amount": -10500,
				"account_id": "a1", "account_name": "Checking",
				"cleared": "uncleared", "approved": false,
			},
		})
	})

	memo := "coffee"
	txn := SaveTransaction{
		AccountID: "a1",
		Date:      "2024-03-15",
		Amount:    -10500,
		Memo:      &memo,
	}
	created, err := client.CreateTransaction(context.Background(), "budget-1", txn)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "new-1" {
		t.Errorf("created id = %q", created.ID)
	}
	if gotBody.Transaction.AccountID != "a1" {
		t.Errorf("request body account_id = %q", gotBody.Transaction.AccountID)
	}
	if gotBody.Transaction.Amount != -10500 {
		t.Errorf("request body amount = %d, want -10500", gotBody.Transaction.Amount)
	}
}

func TestCreateSplitTransaction(t *testing.T) {
	var gotBody SaveTransactionWrapper
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/budgets/budget-1/transactions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		jsonResponse(t, w, map[string]any{
			"transaction": map[string]any{
				"id": "new-1", "date": "2024-03-15", "amount": -10500,
				"account_id": "a1", "account_name": "Checking",
				"cleared": "uncleared", "approved": false,
				"subtransactions": []map[string]any{
					{"id": "sub-1", "amount": -7000, "category_id": "cat-1"},
					{"id": "sub-2", "amount": -3500, "category_id": "cat-2"},
				},
			},
		})
	})

	cat1 := "cat-1"
	cat2 := "cat-2"
	txn := SaveTransaction{
		AccountID:  "a1",
		Date:       "2024-03-15",
		Amount:     -10500,
		CategoryID: nil,
		SubTransactions: []SaveSubTransaction{
			{Amount: -7000, CategoryID: &cat1},
			{Amount: -3500, CategoryID: &cat2},
		},
	}
	created, err := client.CreateTransaction(context.Background(), "budget-1", txn)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "new-1" {
		t.Errorf("created id = %q", created.ID)
	}
	if gotBody.Transaction.CategoryID != nil {
		t.Errorf("parent category_id = %v, want nil", gotBody.Transaction.CategoryID)
	}
	if len(gotBody.Transaction.SubTransactions) != 2 {
		t.Fatalf("got %d subtransactions, want 2", len(gotBody.Transaction.SubTransactions))
	}
	if gotBody.Transaction.SubTransactions[0].Amount != -7000 {
		t.Errorf("first split amount = %d, want -7000", gotBody.Transaction.SubTransactions[0].Amount)
	}
	if gotBody.Transaction.SubTransactions[1].CategoryID == nil || *gotBody.Transaction.SubTransactions[1].CategoryID != "cat-2" {
		t.Errorf("second split category_id = %v, want cat-2", gotBody.Transaction.SubTransactions[1].CategoryID)
	}
}

func TestUpdateTransaction(t *testing.T) {
	var gotBody SaveTransactionWrapper
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		if r.URL.Path != "/budgets/budget-1/transactions/txn-1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)

		jsonResponse(t, w, map[string]any{
			"transaction": map[string]any{
				"id": "txn-1", "date": "2024-03-15", "amount": -10500,
				"account_id": "a1", "account_name": "Checking",
				"category_id": "cat-1", "cleared": "cleared", "approved": true,
			},
		})
	})

	catID := "cat-1"
	txn := SaveTransaction{
		AccountID:  "a1",
		Date:       "2024-03-15",
		Amount:     -10500,
		CategoryID: &catID,
	}
	updated, err := client.UpdateTransaction(context.Background(), "budget-1", "txn-1", txn)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != "txn-1" {
		t.Errorf("updated id = %q", updated.ID)
	}
	if gotBody.Transaction.CategoryID == nil || *gotBody.Transaction.CategoryID != "cat-1" {
		t.Errorf("request body category_id = %v", gotBody.Transaction.CategoryID)
	}
}

func TestGetMonth(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/budgets/budget-1/months/2024-03-01" {
			t.Errorf("path = %q", r.URL.Path)
		}
		jsonResponse(t, w, map[string]any{
			"month": map[string]any{
				"month":          "2024-03-01",
				"income":         5000000,
				"budgeted":       4500000,
				"activity":       -3200000,
				"to_be_budgeted": 500000,
				"age_of_money":   45,
				"categories":     []any{},
			},
		})
	})

	month, err := client.GetMonth(context.Background(), "budget-1", "2024-03-01")
	if err != nil {
		t.Fatal(err)
	}
	if month.Income != 5000000 {
		t.Errorf("income = %d", month.Income)
	}
	if month.AgeOfMoney == nil || *month.AgeOfMoney != 45 {
		t.Errorf("age_of_money = %v", month.AgeOfMoney)
	}
}

func TestAPIError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"id":     "400",
				"name":   "bad_request",
				"detail": "Invalid budget ID",
			},
		})
	})

	_, err := client.ListBudgets(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Name != "bad_request" {
		t.Errorf("error name = %q", apiErr.Name)
	}
	if apiErr.Detail != "Invalid budget ID" {
		t.Errorf("error detail = %q", apiErr.Detail)
	}
}

func TestRateLimitError(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.ListBudgets(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.ID != "429" {
		t.Errorf("error id = %q, want 429", apiErr.ID)
	}
}

func TestRawRateLimitErrorDoesNotRetry(t *testing.T) {
	requests := 0
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, err := client.RawListBudgets(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.ID != "429" {
		t.Errorf("error id = %q, want 429", apiErr.ID)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
}

func TestNon200NoErrorField(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"data": nil})
	})

	_, err := client.ListBudgets(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestInvalidJSON(t *testing.T) {
	client, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json at all"))
	})

	_, err := client.ListBudgets(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
