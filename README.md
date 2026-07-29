# ynab-mcp

An [MCP](https://modelcontextprotocol.io/) server that exposes [YNAB](https://www.ynab.com/) budget operations as tools for Claude Desktop, Claude Code, and other MCP clients. The repository also includes a read-only CLI for shell use and redirected output.

## Prerequisites

- Go 1.26+
- A [YNAB Personal Access Token](https://api.ynab.com/#personal-access-tokens)

## Build

```bash
go build -o ynab-mcp .
go build -o ynab-cli ./cmd/ynab-cli
```

## Configuration

Create `~/.config/ynab-mcp/config`:

```
YNAB_API_TOKEN=your-personal-access-token
YNAB_BUDGET_ID=your-budget-id
```

`YNAB_BUDGET_ID` is optional — it defaults to `last-used`, which picks your most recently modified budget.

Environment variables override the config file, so you can always run with:

```bash
YNAB_API_TOKEN=... ./ynab-mcp
```

Both `ynab-mcp` and `ynab-cli` use this configuration.

## CLI usage

The CLI writes successful results to stdout and errors to stderr, so its output
can be redirected normally:

```bash
./ynab-cli list-transactions \
  --since-date 2025-04-06 \
  --until-date 2026-04-05 \
  --format raw-json > transactions.json
```

Available commands:

```text
list-budgets
get-budget
list-accounts
list-categories
list-payees
list-transactions
get-transaction
get-month-summary
```

Use `./ynab-cli <command> -h` to see command-specific flags. Commands that
operate on a budget use `YNAB_BUDGET_ID` by default and accept
`--budget-id` as an override. The CLI is intentionally read-only; transaction
creation and updates remain available through MCP.

Every read-only command accepts `--format text|raw-json`; `text` is the
default. `raw-json` writes the complete API response body without re-encoding
it. This preserves response-wrapper fields such as `server_knowledge`, unknown
fields, nulls, deleted records, transfers, and subtransactions. Raw output uses
exactly one API request.

`list-transactions` accepts inclusive `--since-date` and `--until-date`
filters. Record the CLI version alongside snapshots with:

```bash
./ynab-cli --version
```

The version is emitted as a single JSON object.

## Usage with Claude Code

```bash
claude mcp add ynab -- /path/to/ynab-mcp
```

## Usage with Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "ynab": {
      "command": "/path/to/ynab-mcp"
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `list_budgets` | List all budgets |
| `get_budget` | Budget details including currency and date range |
| `list_accounts` | Accounts with balances |
| `list_categories` | Category groups with budgeted/activity/balance |
| `list_payees` | Known payees |
| `list_transactions` | Transactions with filters (date, account, category, payee, type) |
| `get_transaction` | Single transaction detail |
| `create_transaction` | Create a new transaction, including split transactions |
| `update_transaction` | Update/categorize a transaction, including converting to a split |
| `get_month_summary` | Monthly budget overview with per-category breakdown |

## Testing

```bash
go test ./...
```

## Rate Limits

The YNAB API allows 200 requests per hour in a rolling window. Both binaries
return a non-zero error on rate-limit hits and do not retry automatically.
