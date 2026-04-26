package ynab

import "fmt"

type APIError struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("YNAB API error %s (%s): %s", e.ID, e.Name, e.Detail)
}
