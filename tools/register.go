package tools

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/venky/ynab-mcp/ynab"
)

func RegisterAll(server *mcp.Server, client *ynab.Client) {
	registerBudgetTools(server, client)
	registerAccountTools(server, client)
	registerCategoryTools(server, client)
	registerPayeeTools(server, client)
	registerTransactionTools(server, client)
	registerMonthTools(server, client)
}

var (
	currencyCache   = make(map[string]*ynab.CurrencyFormat)
	currencyCacheMu sync.Mutex
)

func getCurrencyFormat(ctx context.Context, client *ynab.Client, budgetID string) (*ynab.CurrencyFormat, error) {
	currencyCacheMu.Lock()
	if cf, ok := currencyCache[budgetID]; ok {
		currencyCacheMu.Unlock()
		return cf, nil
	}
	currencyCacheMu.Unlock()

	budget, err := client.GetBudget(ctx, budgetID)
	if err != nil {
		return nil, err
	}

	currencyCacheMu.Lock()
	currencyCache[budgetID] = budget.CurrencyFormat
	currencyCacheMu.Unlock()

	return budget.CurrencyFormat, nil
}
