package processor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
	"github.com/2m-topup/worker/processor"
)

// mockRepository implements payment.Repository for testing.
type mockRepository struct {
	createPaymentFn        func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error)
	getPaymentByIDFn       func(ctx context.Context, id int64) (*payment.Payment, error)
	getPaymentByIdempKeyFn func(ctx context.Context, userID int64, key string) (*payment.Payment, error)
	processWebhookTxFn     func(ctx context.Context, pay *payment.Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (bool, error)
	updateGatewayRefFn     func(ctx context.Context, id int64, gatewayRef string) error
	updateStatusFn         func(ctx context.Context, id int64, status string) error
	updateStatusWithFailFn func(ctx context.Context, id int64, status string, failureReason string) error
}

func (m *mockRepository) CreatePayment(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
	return m.createPaymentFn(ctx, userID, amount, gatewayRef, idempotencyKey, method, expiresAt)
}

func (m *mockRepository) GetUserEmail(ctx context.Context, userID int64) (string, error) {
	return "test@example.com", nil
}

func (m *mockRepository) GetPaymentByID(ctx context.Context, id int64) (*payment.Payment, error) {
	return m.getPaymentByIDFn(ctx, id)
}

func (m *mockRepository) GetPaymentByIdempotencyKey(ctx context.Context, userID int64, key string) (*payment.Payment, error) {
	if m.getPaymentByIdempKeyFn != nil {
		return m.getPaymentByIdempKeyFn(ctx, userID, key)
	}
	return nil, nil
}

func (m *mockRepository) ProcessWebhookTx(ctx context.Context, pay *payment.Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (bool, error) {
	return m.processWebhookTxFn(ctx, pay, gatewayRef, eventID, eventType, targetStatus)
}

func (m *mockRepository) UpdateGatewayReference(ctx context.Context, id int64, gatewayRef string) error {
	if m.updateGatewayRefFn != nil {
		return m.updateGatewayRefFn(ctx, id, gatewayRef)
	}
	return nil
}

func (m *mockRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockRepository) UpdateStatusWithFailure(ctx context.Context, id int64, status string, failureReason string) error {
	if m.updateStatusWithFailFn != nil {
		return m.updateStatusWithFailFn(ctx, id, status, failureReason)
	}
	return nil
}

// mockCache implements cache.Cache for testing.
type mockCache struct {
	getFn    func(ctx context.Context, key string) (string, error)
	setFn    func(ctx context.Context, key string, value string, ttl time.Duration) error
	deleteFn func(ctx context.Context, key string) error
	setNXFn  func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

func (m *mockCache) Get(ctx context.Context, key string) (string, error) {
	return m.getFn(ctx, key)
}

func (m *mockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return m.setFn(ctx, key, value, ttl)
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, key)
	}
	return nil
}

func (m *mockCache) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return m.setNXFn(ctx, key, value, ttl)
}

// TestProcessWebhookJob_Success verifies that a pending payment is successfully processed:
// distributed lock is acquired, transaction is executed, and wallet cache is invalidated.
func TestProcessWebhookJob_Success(t *testing.T) {
	pay := &payment.Payment{
		ID:        1,
		UserID:    123,
		Amount:    500.0,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	txExecuted := false
	deletedKeys := make(map[string]bool)

	repo := &mockRepository{
		getPaymentByIDFn: func(ctx context.Context, id int64) (*payment.Payment, error) {
			if id != 1 {
				t.Fatalf("expected payment ID 1, got %d", id)
			}
			return pay, nil
		},
		processWebhookTxFn: func(ctx context.Context, p *payment.Payment, ref string, eventID string, eventType string, targetStatus string) (bool, error) {
			if ref != "GW-1234" {
				t.Errorf("expected gateway reference GW-1234, got %s", ref)
			}
			if targetStatus != "SUCCESS" {
				t.Errorf("expected target status SUCCESS, got %s", targetStatus)
			}
			txExecuted = true
			return true, nil
		},
	}

	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
			if key != "lock:payment:PAY000001" {
				t.Errorf("expected lock key lock:payment:PAY000001, got %s", key)
			}
			return true, nil
		},
		deleteFn: func(ctx context.Context, key string) error {
			deletedKeys[key] = true
			return nil
		},
	}

	proc := processor.NewPaymentProcessor(repo, cache)
	job := queue.WebhookJob{
		PaymentID:        "PAY000001",
		Status:           "SUCCESS",
		GatewayReference: "GW-1234",
		ReceivedAt:       time.Now(),
	}

	err := proc.ProcessWebhookJob(context.Background(), job)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !txExecuted {
		t.Error("expected database transaction to be executed")
	}

	if !deletedKeys["lock:payment:PAY000001"] {
		t.Error("expected lock to be released (deleted)")
	}

	if !deletedKeys["wallet:balance:123"] {
		t.Error("expected wallet balance cache to be invalidated (deleted)")
	}
}

// TestProcessWebhookJob_Duplicate verifies that if the payment is already processed (not PENDING),
// the processor skips execution and returns ErrPaymentNotPending without executing the database transaction.
func TestProcessWebhookJob_Duplicate(t *testing.T) {
	pay := &payment.Payment{
		ID:        1,
		UserID:    123,
		Amount:    500.0,
		Status:    "SUCCESS", // Already processed!
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	txExecuted := false

	repo := &mockRepository{
		getPaymentByIDFn: func(ctx context.Context, id int64) (*payment.Payment, error) {
			return pay, nil
		},
		processWebhookTxFn: func(ctx context.Context, p *payment.Payment, ref string, eventID string, eventType string, targetStatus string) (bool, error) {
			txExecuted = true
			return true, nil
		},
	}

	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
			t.Error("lock should not be acquired for already processed payments")
			return true, nil
		},
	}

	proc := processor.NewPaymentProcessor(repo, cache)
	job := queue.WebhookJob{
		PaymentID:        "PAY000001",
		Status:           "SUCCESS",
		GatewayReference: "GW-1234",
		ReceivedAt:       time.Now(),
	}

	err := proc.ProcessWebhookJob(context.Background(), job)
	if !errors.Is(err, processor.ErrPaymentNotPending) {
		t.Fatalf("expected ErrPaymentNotPending, got %v", err)
	}

	if txExecuted {
		t.Error("expected database transaction NOT to be executed")
	}
}

// TestProcessWebhookJob_LockFailed verifies that if the distributed lock cannot be acquired,
// the processor returns ErrLockNotAcquired and skips the transaction, indicating a retry is needed.
func TestProcessWebhookJob_LockFailed(t *testing.T) {
	pay := &payment.Payment{
		ID:        1,
		UserID:    123,
		Amount:    500.0,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	txExecuted := false

	repo := &mockRepository{
		getPaymentByIDFn: func(ctx context.Context, id int64) (*payment.Payment, error) {
			return pay, nil
		},
		processWebhookTxFn: func(ctx context.Context, p *payment.Payment, ref string, eventID string, eventType string, targetStatus string) (bool, error) {
			txExecuted = true
			return true, nil
		},
	}

	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, val string, ttl time.Duration) (bool, error) {
			// Fail to acquire lock
			return false, nil
		},
	}

	proc := processor.NewPaymentProcessor(repo, cache)
	job := queue.WebhookJob{
		PaymentID:        "PAY000001",
		Status:           "SUCCESS",
		GatewayReference: "GW-1234",
		ReceivedAt:       time.Now(),
	}

	err := proc.ProcessWebhookJob(context.Background(), job)
	if !errors.Is(err, processor.ErrLockNotAcquired) {
		t.Fatalf("expected ErrLockNotAcquired, got %v", err)
	}

	if txExecuted {
		t.Error("expected database transaction NOT to be executed")
	}
}
