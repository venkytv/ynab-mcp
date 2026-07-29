package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/venky/ynab-mcp/ynab"
)

func testClient(t *testing.T, handler http.Handler) *ynab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return ynab.NewClient("test-token", "test-budget", ynab.WithBaseURL(server.URL))
}

func handleTestBudget(mux *http.ServeMux) {
	mux.HandleFunc("/budgets/test-budget", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"budget": map[string]any{
					"id":   "test-budget",
					"name": "Test Budget",
					"currency_format": map[string]any{
						"iso_code":          "GBP",
						"currency_symbol":   "£",
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
}

func TestRunListTransactionsWritesText(t *testing.T) {
	mux := http.NewServeMux()
	handleTestBudget(mux)
	var sinceDate string
	mux.HandleFunc("/budgets/test-budget/transactions", func(w http.ResponseWriter, r *http.Request) {
		sinceDate = r.URL.Query().Get("since_date")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"transactions": []map[string]any{
					{
						"id":            "txn-1",
						"date":          "2026-07-20",
						"amount":        -4500,
						"account_id":    "account-1",
						"account_name":  "Current Account",
						"payee_name":    "Coffee Shop",
						"category_name": "Dining Out",
						"cleared":       "cleared",
						"approved":      true,
					},
				},
			},
		})
	})

	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		testClient(t, mux),
		[]string{"list-transactions", "--since-date", "2026-07-01"},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if sinceDate != "2026-07-01" {
		t.Errorf("since_date = %q, want 2026-07-01", sinceDate)
	}
	for _, want := range []string{"Coffee Shop", "-£4.50", "txn-1", "Total: 1 transactions"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunRawCommandsPreserveResponseWithOneRequest(t *testing.T) {
	rawResponse := []byte("{\n  \"data\": {\"unknown\": null, \"server_knowledge\": 91}\n}")
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantSince string
		wantUntil string
	}{
		{
			name:     "list budgets",
			args:     []string{"list-budgets", "--format", "raw-json"},
			wantPath: "/budgets",
		},
		{
			name:     "get budget",
			args:     []string{"get-budget", "--format", "raw-json"},
			wantPath: "/budgets/test-budget",
		},
		{
			name:     "list accounts",
			args:     []string{"list-accounts", "--format", "raw-json"},
			wantPath: "/budgets/test-budget/accounts",
		},
		{
			name:     "list categories",
			args:     []string{"list-categories", "--format", "raw-json"},
			wantPath: "/budgets/test-budget/categories",
		},
		{
			name:     "list payees",
			args:     []string{"list-payees", "--format", "raw-json"},
			wantPath: "/budgets/test-budget/payees",
		},
		{
			name: "list transactions",
			args: []string{
				"list-transactions",
				"--format", "raw-json",
				"--since-date", "2025-04-06",
				"--until-date", "2026-04-05",
			},
			wantPath:  "/budgets/test-budget/transactions",
			wantSince: "2025-04-06",
			wantUntil: "2026-04-05",
		},
		{
			name: "get transaction",
			args: []string{
				"get-transaction",
				"--format", "raw-json",
				"--transaction-id", "txn-1",
			},
			wantPath: "/budgets/test-budget/transactions/txn-1",
		},
		{
			name: "get month summary",
			args: []string{
				"get-month-summary",
				"--format", "raw-json",
				"--month", "current",
			},
			wantPath: "/budgets/test-budget/months/current",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != tt.wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, tt.wantPath)
				}
				if got := r.URL.Query().Get("since_date"); got != tt.wantSince {
					t.Errorf("since_date = %q, want %q", got, tt.wantSince)
				}
				if got := r.URL.Query().Get("until_date"); got != tt.wantUntil {
					t.Errorf("until_date = %q, want %q", got, tt.wantUntil)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write(rawResponse)
			}))

			var stdout, stderr bytes.Buffer
			err := run(context.Background(), client, tt.args, &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stdout.Bytes(), rawResponse) {
				t.Errorf("raw output changed:\ngot:  %q\nwant: %q", stdout.Bytes(), rawResponse)
			}
			if requests != 1 {
				t.Errorf("requests = %d, want 1", requests)
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %s", stderr.String())
			}
		})
	}
}

func TestRunListTransactionsRejectsCompetingFilters(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		ynab.NewClient("test-token", "test-budget"),
		[]string{
			"list-transactions",
			"--account-id", "account-1",
			"--payee-id", "payee-1",
		},
		&stdout,
		&stderr,
	)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutually exclusive filter error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunRejectsUnknownFormatWithoutRequest(t *testing.T) {
	requests := 0
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))

	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		client,
		[]string{"list-accounts", "--format", "yaml"},
		&stdout,
		&stderr,
	)
	if err == nil || !strings.Contains(err.Error(), "text or raw-json") {
		t.Fatalf("error = %v, want format validation error", err)
	}
	if requests != 0 {
		t.Errorf("requests = %d, want 0", requests)
	}
}

func TestRunListAccountsUsesBudgetCurrency(t *testing.T) {
	mux := http.NewServeMux()
	handleTestBudget(mux)
	mux.HandleFunc("/budgets/test-budget/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"accounts": []map[string]any{
					{
						"id":        "account-1",
						"name":      "Current Account",
						"type":      "checking",
						"on_budget": true,
						"balance":   123450,
					},
				},
			},
		})
	})

	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		testClient(t, mux),
		[]string{"list-accounts"},
		&stdout,
		&stderr,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "£123.45") {
		t.Errorf("stdout missing formatted GBP amount:\n%s", stdout.String())
	}
}

func TestRunReturnsAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/budgets/test-budget/payees", func(w http.ResponseWriter, r *http.Request) {
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

	var stdout, stderr bytes.Buffer
	err := run(
		context.Background(),
		testClient(t, mux),
		[]string{"list-payees"},
		&stdout,
		&stderr,
	)
	if err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("error = %v, want YNAB API error", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestRealMainHelpDoesNotRequireToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("YNAB_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	exitCode := realMain(context.Background(), []string{"help"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "list-transactions") {
		t.Errorf("help output missing commands:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

func TestRealMainVersionIsMachineReadable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("YNAB_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	exitCode := realMain(context.Background(), []string{"--version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	want := "{\"name\":\"ynab-cli\",\"version\":\"0.2.0\"}\n"
	if stdout.String() != want {
		t.Errorf("version output = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %s", stderr.String())
	}
}

func TestRealMainCommandHelpDoesNotRequireToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("YNAB_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	exitCode := realMain(
		context.Background(),
		[]string{"list-transactions", "-h"},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stderr.String(), "since-date") {
		t.Errorf("command help missing flags:\n%s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestRealMainRequiresToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("YNAB_API_TOKEN", "")

	var stdout, stderr bytes.Buffer
	exitCode := realMain(context.Background(), []string{"list-budgets"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Errorf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "YNAB_API_TOKEN") {
		t.Errorf("stderr missing credential error: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunAndReportRawRateLimitIsNonZeroAndDoesNotRetry(t *testing.T) {
	requests := 0
	client := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	var stdout, stderr bytes.Buffer
	exitCode := runAndReport(
		context.Background(),
		client,
		[]string{"list-budgets", "--format", "raw-json"},
		&stdout,
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "too_many_requests") {
		t.Errorf("stderr missing rate-limit error: %s", stderr.String())
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1", requests)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %s", stdout.String())
	}
}
