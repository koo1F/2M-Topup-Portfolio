package wallet

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository is the interface for wallet-related data access.
type Repository interface {
	// GetBalance returns the wallet balance for the given user.
	GetBalance(ctx context.Context, userID int64) (float64, error)
	// GetTransactions returns a paginated list of transactions and the total count.
	GetTransactions(ctx context.Context, userID int64, page, limit int) ([]Transaction, int, error)
}

type pgRepository struct {
	db *sql.DB
}

// NewRepository creates a PostgreSQL-backed wallet Repository.
func NewRepository(db *sql.DB) Repository {
	return &pgRepository{db: db}
}

func (r *pgRepository) GetBalance(ctx context.Context, userID int64) (float64, error) {
	const query = `SELECT balance FROM wallets WHERE user_id = $1`

	var balance float64
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("get wallet balance: %w", err)
	}
	return balance, nil
}

func (r *pgRepository) GetTransactions(ctx context.Context, userID int64, page, limit int) ([]Transaction, int, error) {
	offset := (page - 1) * limit

	// Total count for pagination metadata
	const countQuery = `
		SELECT COUNT(*)
		FROM transactions t
		JOIN wallets w ON t.wallet_id = w.id
		WHERE w.user_id = $1`

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	// Paginated results — newest first
	const dataQuery = `
		SELECT t.id, t.type, t.amount, t.status, t.reference_id, t.created_at
		FROM transactions t
		JOIN wallets w ON t.wallet_id = w.id
		WHERE w.user_id = $1
		ORDER BY t.created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, dataQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query transactions: %w", err)
	}
	defer rows.Close()

	txs := make([]Transaction, 0)
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.Type, &tx.Amount, &tx.Status, &tx.ReferenceID, &tx.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan transaction row: %w", err)
		}
		txs = append(txs, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate transactions: %w", err)
	}

	return txs, total, nil
}
