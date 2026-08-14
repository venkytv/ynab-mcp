# CLAUDE.md

## What this is

Stdio MCP server in Go that exposes YNAB budget operations as tools. Used with Claude Desktop, Claude Code, and other MCP clients. Includes a separate read-only CLI for shell use and redirected output.

## Build and test

```bash
go build -o ynab-mcp .
go build -o ynab-cli ./cmd/ynab-cli
go test ./...
```

## Architecture

- `main.go` — Entrypoint: loads config, creates YNAB client, registers tools, runs stdio server
- `cmd/ynab-cli/` — Read-only shell CLI with text output on stdout
- `internal/config/` — Shared config loader for `~/.config/ynab-mcp/config` (KEY=VALUE); env vars take precedence
- `ynab/` — HTTP client for the YNAB API (`api.ynab.com/v1`), types, error handling, currency formatting
- `tools/` — MCP tool handlers, one file per resource (budgets, accounts, categories, payees, transactions, months)
- `tools/register.go` — Wires all tools to the MCP server; manages per-budget currency format cache
- `tools/helpers.go` — Shared utilities (`resolveBudgetID`, `errorResult`, `textResult`)

## Key conventions

- **Currency formatting**: Always use `ynab.FormatAmount()` with the budget's `CurrencyFormat`. Never hardcode "$" or any currency symbol.
- **Milliunits**: YNAB API uses milliunits (1000 = $1.00). Tool inputs accept human amounts (e.g. -10.50); convert with `ynab.DollarsToMilliunits()`.
- **Tool errors**: Return `IsError: true` via `errorResult()`, not protocol-level errors. The LLM needs to see and reason about errors.
- **Text output**: Tools return formatted text, not JSON. LLMs parse prose better and it uses fewer tokens.
- **Raw CLI output**: `--format raw-json` writes the API response bytes unchanged and makes exactly one request. Do not decode and re-encode it or perform a currency lookup first.
- **Budget ID fallback**: All tools accept optional `budget_id`; `resolveBudgetID()` falls back to the client's configured default.
- **Closure-based DI**: `ynab.Client` is passed to tool handlers via closure capture in registration functions. No globals, no interfaces.

## Testing patterns

- `ynab/client_test.go` — Tests the HTTP client against `httptest.NewServer` mocks. Each test creates its own server for isolation. `WithBaseURL()` points the client at the mock.
- `tools/tools_test.go` — Full MCP round-trip tests: MCP client → server → tool handler → ynab.Client → httptest mock. Uses `setupTestEnv()` which resets the package-level `currencyCache` between tests.
- `main_test.go` — Verifies all 10 tools register correctly.

## YNAB API

- Base URL: `https://api.ynab.com/v1`
- Auth: Bearer token via `YNAB_API_TOKEN`
- Rate limit: 200 requests/hour
- `list_transactions` filters by path (account/category/payee) and query params (`since_date`, `until_date`, `type`). The date bounds are inclusive. The `type` param supports `uncategorized` and `unapproved` only — there is no server-side filter for cleared status.
