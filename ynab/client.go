package ynab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	budgetID   string
}

type ClientOption func(*Client)

func WithHTTPClient(c *http.Client) ClientOption {
	return func(cl *Client) { cl.httpClient = c }
}

func WithBaseURL(u string) ClientOption {
	return func(cl *Client) { cl.baseURL = u }
}

func NewClient(token, budgetID string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: http.DefaultClient,
		baseURL:    "https://api.ynab.com/v1",
		token:      token,
		budgetID:   budgetID,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) BudgetID() string {
	return c.budgetID
}

type rawResponse struct {
	Data  json.RawMessage `json:"data"`
	Error *APIError       `json:"error"`
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body any, result any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return &APIError{
			ID:     "429",
			Name:   "too_many_requests",
			Detail: "YNAB API rate limit exceeded (200 requests/hour). Please wait before making more requests.",
		}
	}

	var raw rawResponse
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return fmt.Errorf("status %d: failed to parse response: %w", resp.StatusCode, err)
	}
	if raw.Error != nil {
		return raw.Error
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("YNAB API returned status %d", resp.StatusCode)
	}

	if result != nil && raw.Data != nil {
		if err := json.Unmarshal(raw.Data, result); err != nil {
			return fmt.Errorf("unmarshaling response data: %w", err)
		}
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	return c.doRequest(ctx, http.MethodGet, path, query, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	return c.doRequest(ctx, http.MethodPost, path, nil, body, result)
}

func (c *Client) put(ctx context.Context, path string, body any, result any) error {
	return c.doRequest(ctx, http.MethodPut, path, nil, body, result)
}

// --- Budgets ---

func (c *Client) ListBudgets(ctx context.Context) ([]BudgetSummary, error) {
	var data BudgetListData
	if err := c.get(ctx, "/budgets", nil, &data); err != nil {
		return nil, err
	}
	return data.Budgets, nil
}

func (c *Client) GetBudget(ctx context.Context, budgetID string) (*BudgetSummary, error) {
	var data BudgetDetailData
	if err := c.get(ctx, "/budgets/"+budgetID, nil, &data); err != nil {
		return nil, err
	}
	return &data.Budget, nil
}

// --- Accounts ---

func (c *Client) ListAccounts(ctx context.Context, budgetID string) ([]Account, error) {
	var data AccountListData
	if err := c.get(ctx, "/budgets/"+budgetID+"/accounts", nil, &data); err != nil {
		return nil, err
	}
	return data.Accounts, nil
}

// --- Categories ---

func (c *Client) ListCategories(ctx context.Context, budgetID string) ([]CategoryGroup, error) {
	var data CategoryListData
	if err := c.get(ctx, "/budgets/"+budgetID+"/categories", nil, &data); err != nil {
		return nil, err
	}
	return data.CategoryGroups, nil
}

// --- Payees ---

func (c *Client) ListPayees(ctx context.Context, budgetID string) ([]Payee, error) {
	var data PayeeListData
	if err := c.get(ctx, "/budgets/"+budgetID+"/payees", nil, &data); err != nil {
		return nil, err
	}
	return data.Payees, nil
}

// --- Transactions ---

func (c *Client) ListTransactions(ctx context.Context, budgetID string, opts *ListTransactionsOptions) ([]TransactionDetail, error) {
	path := "/budgets/" + budgetID
	if opts != nil {
		switch {
		case opts.AccountID != "":
			path += "/accounts/" + opts.AccountID + "/transactions"
		case opts.CategoryID != "":
			path += "/categories/" + opts.CategoryID + "/transactions"
		case opts.PayeeID != "":
			path += "/payees/" + opts.PayeeID + "/transactions"
		default:
			path += "/transactions"
		}
	} else {
		path += "/transactions"
	}

	query := url.Values{}
	if opts != nil {
		if opts.SinceDate != "" {
			query.Set("since_date", opts.SinceDate)
		}
		if opts.Type != "" {
			query.Set("type", opts.Type)
		}
	}

	var data TransactionListData
	if err := c.get(ctx, path, query, &data); err != nil {
		return nil, err
	}
	return data.Transactions, nil
}

func (c *Client) GetTransaction(ctx context.Context, budgetID, transactionID string) (*TransactionDetail, error) {
	var data TransactionData
	if err := c.get(ctx, "/budgets/"+budgetID+"/transactions/"+transactionID, nil, &data); err != nil {
		return nil, err
	}
	return &data.Transaction, nil
}

func (c *Client) CreateTransaction(ctx context.Context, budgetID string, txn SaveTransaction) (*TransactionDetail, error) {
	wrapper := SaveTransactionWrapper{Transaction: txn}
	var data SaveTransactionsResponseData
	if err := c.post(ctx, "/budgets/"+budgetID+"/transactions", wrapper, &data); err != nil {
		return nil, err
	}
	return data.Transaction, nil
}

func (c *Client) UpdateTransaction(ctx context.Context, budgetID, transactionID string, txn SaveTransaction) (*TransactionDetail, error) {
	wrapper := SaveTransactionWrapper{Transaction: txn}
	var data TransactionData
	if err := c.put(ctx, "/budgets/"+budgetID+"/transactions/"+transactionID, wrapper, &data); err != nil {
		return nil, err
	}
	return &data.Transaction, nil
}

// --- Months ---

func (c *Client) GetMonth(ctx context.Context, budgetID, month string) (*MonthDetail, error) {
	var data MonthDetailData
	if err := c.get(ctx, "/budgets/"+budgetID+"/months/"+month, nil, &data); err != nil {
		return nil, err
	}
	return &data.Month, nil
}
