package wallet_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/2m-topup/backend/pkg/middleware"
	"github.com/2m-topup/backend/pkg/wallet"
)

// --- Mock Service ---

type mockService struct {
	getBalanceFn      func(ctx context.Context, userID int64) (*wallet.BalanceResponse, error)
	getTransactionsFn func(ctx context.Context, userID int64, page, limit int) (*wallet.TransactionsResponse, error)
}

func (m *mockService) GetBalance(ctx context.Context, userID int64) (*wallet.BalanceResponse, error) {
	return m.getBalanceFn(ctx, userID)
}

func (m *mockService) GetTransactions(ctx context.Context, userID int64, page, limit int) (*wallet.TransactionsResponse, error) {
	return m.getTransactionsFn(ctx, userID, page, limit)
}

// --- GetBalance tests ---

func TestGetBalance_Success(t *testing.T) {
	svc := &mockService{
		getBalanceFn: func(_ context.Context, userID int64) (*wallet.BalanceResponse, error) {
			return &wallet.BalanceResponse{Balance: 500.00, Currency: "THB"}, nil
		},
	}
	h := wallet.NewHandler(svc)

	w, r := newAuthRequest(t, http.MethodGet, "/api/wallet", 1)
	h.GetBalance(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp wallet.BalanceResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if resp.Balance != 500.00 {
		t.Errorf("expected balance=500.00, got %f", resp.Balance)
	}
	if resp.Currency != "THB" {
		t.Errorf("expected currency=THB, got %s", resp.Currency)
	}
}

func TestGetBalance_NoAuth(t *testing.T) {
	h := wallet.NewHandler(&mockService{})

	// No userID in context — simulates request that bypassed JWT middleware
	r := httptest.NewRequest(http.MethodGet, "/api/wallet", nil)
	w := httptest.NewRecorder()
	h.GetBalance(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetBalance_ServiceError(t *testing.T) {
	svc := &mockService{
		getBalanceFn: func(_ context.Context, _ int64) (*wallet.BalanceResponse, error) {
			return nil, errors.New("db connection lost")
		},
	}
	h := wallet.NewHandler(svc)

	w, r := newAuthRequest(t, http.MethodGet, "/api/wallet", 1)
	h.GetBalance(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// --- GetTransactions tests ---

func TestGetTransactions_Success(t *testing.T) {
	now := time.Now()
	svc := &mockService{
		getTransactionsFn: func(_ context.Context, userID int64, page, limit int) (*wallet.TransactionsResponse, error) {
			return &wallet.TransactionsResponse{
				Data: []wallet.Transaction{
					{ID: 1, Type: "DEPOSIT", Amount: 100.00, Status: "SUCCESS", ReferenceID: "REF001", CreatedAt: now},
				},
				Pagination: wallet.PaginationMeta{Page: page, Limit: limit, Total: 1},
			}, nil
		},
	}
	h := wallet.NewHandler(svc)

	w, r := newAuthRequest(t, http.MethodGet, "/api/wallet/transactions?page=1&limit=20", 1)
	h.GetTransactions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp wallet.TransactionsResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(resp.Data))
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Pagination.Total)
	}
}

func TestGetTransactions_EmptyList(t *testing.T) {
	svc := &mockService{
		getTransactionsFn: func(_ context.Context, _ int64, page, limit int) (*wallet.TransactionsResponse, error) {
			return &wallet.TransactionsResponse{
				Data:       []wallet.Transaction{},
				Pagination: wallet.PaginationMeta{Page: page, Limit: limit, Total: 0},
			}, nil
		},
	}
	h := wallet.NewHandler(svc)

	w, r := newAuthRequest(t, http.MethodGet, "/api/wallet/transactions", 1)
	h.GetTransactions(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp wallet.TransactionsResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	// Must be an empty array [], not null
	raw, _ := json.Marshal(resp.Data)
	if string(raw) == "null" {
		t.Error("data field must be [] not null")
	}
}

func TestGetTransactions_NoAuth(t *testing.T) {
	h := wallet.NewHandler(&mockService{})

	r := httptest.NewRequest(http.MethodGet, "/api/wallet/transactions", nil)
	w := httptest.NewRecorder()
	h.GetTransactions(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Helpers ---

// newAuthRequest creates a test request with userID injected into context
// (simulates what the JWT middleware does in production).
func newAuthRequest(t *testing.T, method, path string, userID int64) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	return httptest.NewRecorder(), r
}

func mustDecode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode response: %v — raw: %s", err, data)
	}
}
