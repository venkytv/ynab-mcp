# ynab-mcp

An [MCP](https://modelcontextprotocol.io/) server that exposes [YNAB](https://www.ynab.com/) budget operations as tools for Claude Desktop, Claude Code, and other MCP clients.

## Prerequisites

- Go 1.26+
- A [YNAB Personal Access Token](https://api.ynab.com/#personal-access-tokens)

## Build

```bash
go build -o ynab-mcp .
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
| `create_transaction` | Create a new transaction |
| `update_transaction` | Update/categorize a transaction (only changed fields) |
| `get_month_summary` | Monthly budget overview with per-category breakdown |

## Testing

```bash
go test ./...
```

## Rate Limits

The YNAB API allows 200 requests per hour. The server returns clear errors on rate limit hits but does not retry automatically.
