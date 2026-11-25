package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func stubTransferAccounts(env *testEnv) {
	env.mux.HandleFunc("GET /budgets/test-budget/accounts", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"accounts":[
			{"id":"checking","on_budget":true,"transfer_payee_id":"payee-checking"},
			{"id":"savings","on_budget":true,"transfer_payee_id":"payee-savings"},
			{"id":"tracking","on_budget":false,"transfer_payee_id":"payee-tracking"},
			{"id":"deleted","deleted":true,"transfer_payee_id":"payee-deleted"},
			{"id":"no-payee","transfer_payee_id":null}
		]}}`)
	})
}

func stubTransferOriginal(env *testEnv) {
	env.mux.HandleFunc("GET /budgets/test-budget/transactions/txn-1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"transaction":{
			"id":"txn-1","account_id":"checking","date":"2026-10-01","amount":-12500,
			"payee_id":"regular-payee","payee_name":"Bank","category_id":"old-category",
			"memo":"original memo","cleared":"cleared","approved":true,"flag_color":"red"
		}}}`)
	})
}

func TestTransactionTransferTool(t *testing.T) {
	tests := []struct {
		name           string
		update         bool
		otherAccountID string
		categoryID     string
		wantCategory   string
		split          bool
		amount         float64
		wantAmount     int
	}{
		{name: "create outflow", otherAccountID: "savings", wantCategory: "null", amount: -12.5, wantAmount: -12500},
		{name: "create inflow from tracking", otherAccountID: "tracking", categoryID: "new-category", wantCategory: `"new-category"`, amount: 12.5, wantAmount: 12500},
		{name: "create split transfer to tracking", otherAccountID: "tracking", wantCategory: "null", split: true, amount: -12.5, wantAmount: -12500},
		{name: "convert and clear category", update: true, otherAccountID: "savings", wantCategory: "null", wantAmount: -12500},
		{name: "convert and retain category", update: true, otherAccountID: "tracking", wantCategory: `"old-category"`, wantAmount: -12500},
		{name: "convert and replace category", update: true, otherAccountID: "tracking", categoryID: "new-category", wantCategory: `"new-category"`, wantAmount: -12500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestEnv(t)
			stubTransferAccounts(env)
			name := "create_transaction"
			pattern := "POST /budgets/test-budget/transactions"
			args := map[string]any{
				"account_id": "checking", "date": "2026-10-01", "amount": tt.amount,
			}
			if tt.update {
				stubTransferOriginal(env)
				name = "update_transaction"
				pattern = "PUT /budgets/test-budget/transactions/txn-1"
				args = map[string]any{"transaction_id": "txn-1"}
			}
			args["transfer_account_id"] = tt.otherAccountID
			if tt.categoryID != "" {
				args["category_id"] = tt.categoryID
			}
			if tt.split {
				args["subtransactions"] = []map[string]any{
					{"amount": -5, "category_id": "first-category"},
					{"amount": -7.5, "category_id": "second-category"},
				}
			}

			var writes atomic.Int32
			env.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				writes.Add(1)
				env.captureBody(r)
				fmt.Fprintf(w, `{"data":{"transaction":{
					"id":"txn-1","transfer_account_id":%q,"transfer_transaction_id":"counterpart"
				}}}`, tt.otherAccountID)
			})

			result := callTool(t, env, name, args)
			text := toolText(t, result)
			if result.IsError {
				t.Fatalf("unexpected tool error: %s", text)
			}
			for _, want := range []string{"Transfer account ID: " + tt.otherAccountID, "Linked transaction ID: counterpart"} {
				if !strings.Contains(text, want) {
					t.Errorf("result missing %q: %s", want, text)
				}
			}
			if got := writes.Load(); got != 1 {
				t.Errorf("got %d writes, want 1", got)
			}

			env.bodiesMu.Lock()
			body := env.bodies[pattern]
			env.bodiesMu.Unlock()
			var request struct {
				Transaction map[string]json.RawMessage `json:"transaction"`
			}
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			wantFields := map[string]string{
				"account_id":  `"checking"`,
				"date":        `"2026-10-01"`,
				"amount":      strconv.Itoa(tt.wantAmount),
				"payee_id":    strconv.Quote("payee-" + tt.otherAccountID),
				"category_id": tt.wantCategory,
			}
			if tt.update {
				wantFields["memo"] = `"original memo"`
				wantFields["cleared"] = `"cleared"`
				wantFields["approved"] = "true"
				wantFields["flag_color"] = `"red"`
			}
			for key, want := range wantFields {
				if got := string(request.Transaction[key]); got != want {
					t.Errorf("sent %s = %s, want %s", key, got, want)
				}
			}
			for _, key := range []string{"payee_name", "transfer_account_id"} {
				if _, found := request.Transaction[key]; found {
					t.Errorf("unexpected %s in YNAB request: %s", key, body)
				}
			}
			if tt.split {
				var lines []json.RawMessage
				if err := json.Unmarshal(request.Transaction["subtransactions"], &lines); err != nil {
					t.Fatal(err)
				}
				if len(lines) != 2 {
					t.Errorf("sent %d split lines, want 2", len(lines))
				}
			}
		})
	}
}

func TestTransactionTransferToolRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name           string
		update         bool
		otherAccountID string
		extraField     string
		wantError      string
	}{
		{name: "create with payee ID", otherAccountID: "savings", extraField: "payee_id", wantError: "cannot be combined"},
		{name: "update with payee name", update: true, otherAccountID: "savings", extraField: "payee_name", wantError: "cannot be combined"},
		{name: "blank account", otherAccountID: " ", wantError: "non-empty account ID"},
		{name: "self transfer", update: true, otherAccountID: "checking", wantError: "must differ"},
		{name: "unknown account", otherAccountID: "unknown", wantError: `transfer account "unknown" is unavailable`},
		{name: "deleted account", otherAccountID: "deleted", wantError: `transfer account "deleted" is unavailable`},
		{name: "missing transfer payee", otherAccountID: "no-payee", wantError: "has no transfer payee"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupTestEnv(t)
			stubTransferAccounts(env)
			stubTransferOriginal(env)
			for _, pattern := range []string{"POST /budgets/test-budget/transactions", "PUT /budgets/test-budget/transactions/txn-1"} {
				env.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
					t.Errorf("unexpected write: %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				})
			}

			name := "create_transaction"
			args := map[string]any{"account_id": "checking", "date": "2026-10-01", "amount": -12.5}
			if tt.update {
				name = "update_transaction"
				args = map[string]any{"transaction_id": "txn-1"}
			}
			args["transfer_account_id"] = tt.otherAccountID
			if tt.extraField != "" {
				args[tt.extraField] = "regular-payee"
			}

			result := callTool(t, env, name, args)
			if text := toolText(t, result); !result.IsError || !strings.Contains(text, tt.wantError) {
				t.Errorf("expected error containing %q, got %q (IsError=%v)", tt.wantError, text, result.IsError)
			}
		})
	}
}
