package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

var ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")

// Repository is the interface for payment-related data access.
type Repository interface {
	// CreatePayment inserts a new payment row and returns it.
	CreatePayment(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*Payment, error)
	// GetPaymentByID fetches a payment by its numeric ID. Returns nil, nil if not found.
	GetPaymentByID(ctx context.Context, id int64) (*Payment, error)
	// GetPaymentByIdempotencyKey fetches a payment by its idempotency key. Returns nil, nil if not found.
	GetPaymentByIdempotencyKey(ctx context.Context, userID int64, key string) (*Payment, error)
	// UpdateGatewayReference updates the gateway_reference field of a payment.
	UpdateGatewayReference(ctx context.Context, id int64, gatewayRef string) error
	// UpdateStatus updates the status field of a payment.
	UpdateStatus(ctx context.Context, id int64, status string) error
	// UpdateStatusWithFailure updates status and stores the failure reason.
	UpdateStatusWithFailure(ctx context.Context, id int64, status string, failureReason string) error
	// GetUserEmail retrieves a user's email address by their user ID.
	GetUserEmail(ctx context.Context, userID int64) (string, error)
	// ProcessWebhookTx executes the critical-section DB transaction atomically:
	//   SELECT payments FOR UPDATE (row lock)
	//   INSERT processed_events (idempotency check)
	//   UPDATE payments SET status=SUCCESS/EXPIRED
	//   If SUCCESS: INSERT transactions (DEPOSIT) and UPDATE wallets SET balance += amount
	// Returns (false, nil) if the payment was already processed or duplicate event.
	ProcessWebhookTx(ctx context.Context, payment *Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (updated bool, err error)
}

type pgRepository struct {
	db *sql.DB
}

// NewRepository creates a PostgreSQL-backed payment Repository.
func NewRepository(db *sql.DB) Repository {
	return &pgRepository{db: db}
}

func (r *pgRepository) CreatePayment(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*Payment, error) {
	const query = `
		INSERT INTO payments (user_id, amount, gateway_reference, idempotency_key, method, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, amount, status, COALESCE(gateway_reference, ''), idempotency_key, expires_at, created_at, COALESCE(failure_reason, ''), method`

	var p Payment
	err := r.db.QueryRowContext(ctx, query, userID, amount, gatewayRef, idempotencyKey, method, expiresAt).
		Scan(&p.ID, &p.UserID, &p.Amount, &p.Status, &p.GatewayReference, &p.IdempotencyKey, &p.ExpiresAt, &p.CreatedAt, &p.FailureReason, &p.Method)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateIdempotencyKey
		}
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return &p, nil
}

func (r *pgRepository) GetPaymentByID(ctx context.Context, id int64) (*Payment, error) {
	const query = `
		SELECT id, user_id, amount, status, COALESCE(gateway_reference,''), idempotency_key, expires_at, created_at, paid_at, COALESCE(failure_reason, ''), method
		FROM payments
		WHERE id = $1`

	var p Payment
	var paidAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&p.ID, &p.UserID, &p.Amount, &p.Status, &p.GatewayReference,
			&p.IdempotencyKey, &p.ExpiresAt, &p.CreatedAt, &paidAt, &p.FailureReason, &p.Method)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	if paidAt.Valid {
		p.PaidAt = &paidAt.Time
	}
	return &p, nil
}

func (r *pgRepository) GetPaymentByIdempotencyKey(ctx context.Context, userID int64, key string) (*Payment, error) {
	const query = `
		SELECT id, user_id, amount, status, COALESCE(gateway_reference,''), idempotency_key, expires_at, created_at, paid_at, COALESCE(failure_reason, ''), method
		FROM payments
		WHERE user_id = $1 AND idempotency_key = $2`

	var p Payment
	var paidAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID, key).
		Scan(&p.ID, &p.UserID, &p.Amount, &p.Status, &p.GatewayReference,
			&p.IdempotencyKey, &p.ExpiresAt, &p.CreatedAt, &paidAt, &p.FailureReason, &p.Method)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get payment by idempotency key: %w", err)
	}
	if paidAt.Valid {
		p.PaidAt = &paidAt.Time
	}
	return &p, nil
}

func (r *pgRepository) UpdateGatewayReference(ctx context.Context, id int64, gatewayRef string) error {
	const query = `UPDATE payments SET gateway_reference = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, gatewayRef, id)
	if err != nil {
		return fmt.Errorf("update gateway reference: %w", err)
	}
	return nil
}

func (r *pgRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	const query = `UPDATE payments SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

func (r *pgRepository) UpdateStatusWithFailure(ctx context.Context, id int64, status string, failureReason string) error {
	const query = `UPDATE payments SET status = $1, failure_reason = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, failureReason, id)
	if err != nil {
		return fmt.Errorf("update status with failure: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func (r *pgRepository) GetUserEmail(ctx context.Context, userID int64) (string, error) {
	const query = `SELECT email FROM users WHERE id = $1`
	var email string
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&email)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get user email: %w", err)
	}
	return email, nil
}

// ProcessWebhookTx runs the full payment settlement atomically.
// Returns updated=true if the payment was successfully settled.
// Returns updated=false if the payment was not PENDING (already processed) or if event is duplicate.
func (r *pgRepository) ProcessWebhookTx(ctx context.Context, payment *Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// 1. SELECT FOR UPDATE to acquire a row-level lock on the payment
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE id = $1 FOR UPDATE`, payment.ID).Scan(&status)
	if err != nil {
		return false, fmt.Errorf("lock payment row: %w", err)
	}

	if status != "PENDING" {
		// Row is already processed or expired
		return false, nil
	}

	// 2. Stripe Event Idempotency Check (Database Level UNIQUE constraint)
	if eventID != "" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO processed_events (id, event_type)
			VALUES ($1, $2)`, eventID, eventType)
		if err != nil {
			// Only a unique violation means the event was already processed.
			if isUniqueViolation(err) {
				return false, nil
			}
			return false, fmt.Errorf("insert processed event: %w", err)
		}
	}

	paidAt := time.Now()

	if targetStatus == "EXPIRED" {
		// Only update status to EXPIRED and associate gateway_reference, do not credit wallet
		_, err = tx.ExecContext(ctx, `
			UPDATE payments
			SET status = 'EXPIRED', paid_at = $1, gateway_reference = $2
			WHERE id = $3`,
			paidAt, gatewayRef, payment.ID)
		if err != nil {
			return false, fmt.Errorf("update payment status to EXPIRED: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return false, fmt.Errorf("commit EXPIRED tx: %w", err)
		}
		return true, nil
	}

	// 3. Target Status is SUCCESS: update status to SUCCESS
	_, err = tx.ExecContext(ctx, `
		UPDATE payments
		SET status = 'SUCCESS', paid_at = $1, gateway_reference = $2
		WHERE id = $3`,
		paidAt, gatewayRef, payment.ID)
	if err != nil {
		return false, fmt.Errorf("update payment status to SUCCESS: %w", err)
	}

	// Get wallet_id for this user
	var walletID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM wallets WHERE user_id = $1`, payment.UserID).
		Scan(&walletID); err != nil {
		return false, fmt.Errorf("get wallet: %w", err)
	}

	// INSERT transaction record
	// reference_id has a UNIQUE constraint → extra protection against double-insert
	refID := FormatID(payment.ID)
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO transactions (wallet_id, type, amount, status, reference_id)
		VALUES ($1, 'DEPOSIT', $2, 'SUCCESS', $3)`,
		walletID, payment.Amount, refID); err != nil {
		return false, fmt.Errorf("insert transaction: %w", err)
	}

	// UPDATE wallet balance
	if _, err = tx.ExecContext(ctx, `
		UPDATE wallets SET balance = balance + $1, updated_at = NOW()
		WHERE user_id = $2`,
		payment.Amount, payment.UserID); err != nil {
		return false, fmt.Errorf("update wallet: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit SUCCESS tx: %w", err)
	}
	return true, nil
}
