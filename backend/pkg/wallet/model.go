package wallet

import "time"

// Transaction represents a single wallet transaction record.
type Transaction struct {
	ID          int64     `json:"id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	ReferenceID string    `json:"reference_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// BalanceResponse is the response body for GET /api/wallet.
type BalanceResponse struct {
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// PaginationMeta carries pagination metadata in the response.
type PaginationMeta struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// TransactionsResponse is the response body for GET /api/wallet/transactions.
type TransactionsResponse struct {
	Data       []Transaction  `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}
