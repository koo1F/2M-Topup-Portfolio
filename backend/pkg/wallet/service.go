package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/2m-topup/backend/pkg/cache"
)

const (
	balanceCacheTTL = 30 * time.Second
	maxLimit        = 100
	defaultLimit    = 20
)

// Service is the interface for wallet business logic.
type Service interface {
	GetBalance(ctx context.Context, userID int64) (*BalanceResponse, error)
	GetTransactions(ctx context.Context, userID int64, page, limit int) (*TransactionsResponse, error)
}

type service struct {
	repo  Repository
	cache cache.Cache
}

// NewService creates a wallet Service backed by repo and cache.
func NewService(repo Repository, c cache.Cache) Service {
	return &service{repo: repo, cache: c}
}

// GetBalance implements the cache-aside pattern:
//
//  1. Try Redis key "wallet:balance:{user_id}"
//  2. On miss → query Postgres
//  3. Populate cache (TTL = 30s)
//  4. Return balance
func (s *service) GetBalance(ctx context.Context, userID int64) (*BalanceResponse, error) {
	key := balanceKey(userID)

	// ── 1. Cache hit ──────────────────────────────────────────────────────────
	if cached, err := s.cache.Get(ctx, key); err == nil {
		var balance float64
		if json.Unmarshal([]byte(cached), &balance) == nil {
			return &BalanceResponse{Balance: balance, Currency: "THB"}, nil
		}
	}

	// ── 2. Cache miss → Postgres ──────────────────────────────────────────────
	balance, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// ── 3. Populate cache (best-effort; never fail the request) ───────────────
	if data, err := json.Marshal(balance); err == nil {
		_ = s.cache.Set(ctx, key, string(data), balanceCacheTTL)
	}

	return &BalanceResponse{Balance: balance, Currency: "THB"}, nil
}

// GetTransactions returns a paginated list; page/limit are sanitised.
func (s *service) GetTransactions(ctx context.Context, userID int64, page, limit int) (*TransactionsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > maxLimit {
		limit = defaultLimit
	}

	txs, total, err := s.repo.GetTransactions(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	return &TransactionsResponse{
		Data: txs,
		Pagination: PaginationMeta{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	}, nil
}

// balanceKey returns the Redis key used to cache a user's balance.
func balanceKey(userID int64) string {
	return fmt.Sprintf("wallet:balance:%d", userID)
}
