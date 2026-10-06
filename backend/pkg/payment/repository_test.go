package payment_test

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/2m-topup/backend/pkg/payment"
)

func TestProcessWebhookTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := payment.NewRepository(db)

	pay := &payment.Payment{
		ID:        1,
		UserID:    42,
		Amount:    100.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	mock.ExpectBegin()

	// 1. SELECT FOR UPDATE to lock row
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM payments WHERE id = $1 FOR UPDATE")).
		WithArgs(pay.ID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PENDING"))

	// 2. Insert processed event
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO processed_events (id, event_type) VALUES ($1, $2)")).
		WithArgs("evt_123", "checkout.session.completed").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 3. Update payment status
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payments SET status = 'SUCCESS', paid_at = $1, gateway_reference = $2 WHERE id = $3")).
		WithArgs(sqlmock.AnyArg(), "cs_test_123", pay.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Get wallet ID
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM wallets WHERE user_id = $1")).
		WithArgs(pay.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))

	// 5. Insert transaction
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO transactions (wallet_id, type, amount, status, reference_id) VALUES ($1, 'DEPOSIT', $2, 'SUCCESS', $3)")).
		WithArgs(int64(10), pay.Amount, payment.FormatID(pay.ID)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Update wallet balance
	mock.ExpectExec(regexp.QuoteMeta("UPDATE wallets SET balance = balance + $1, updated_at = NOW() WHERE user_id = $2")).
		WithArgs(pay.Amount, pay.UserID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	updated, err := repo.ProcessWebhookTx(context.Background(), pay, "cs_test_123", "evt_123", "checkout.session.completed", "SUCCESS")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !updated {
		t.Errorf("expected updated to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestProcessWebhookTx_DuplicateEvent(t *testing.T) {
	// 1. Duplicate Stripe webhook event ต้องไม่เพิ่ม wallet ซ้ำ
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := payment.NewRepository(db)

	pay := &payment.Payment{
		ID:     1,
		UserID: 42,
		Amount: 100.00,
		Status: "PENDING",
	}

	mock.ExpectBegin()

	// SELECT FOR UPDATE locks the row
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM payments WHERE id = $1 FOR UPDATE")).
		WithArgs(pay.ID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PENDING"))

	// INSERT into processed_events fails due to UNIQUE constraint
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO processed_events (id, event_type) VALUES ($1, $2)")).
		WithArgs("evt_duplicate", "checkout.session.completed").
		WillReturnError(&pq.Error{Code: "23505", Constraint: "processed_events_pkey"})

	mock.ExpectRollback()

	updated, err := repo.ProcessWebhookTx(context.Background(), pay, "cs_test_123", "evt_duplicate", "checkout.session.completed", "SUCCESS")
	if err != nil {
		t.Fatalf("expected no error on duplicate event (idempotent path), got: %v", err)
	}
	if updated {
		t.Errorf("expected updated to be false on duplicate event")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestProcessWebhookTx_AlreadyProcessed(t *testing.T) {
	// 2. Payment status SUCCESS แล้ว webhook ซ้ำต้องไม่สร้าง transaction ใหม่
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := payment.NewRepository(db)

	pay := &payment.Payment{
		ID:     1,
		UserID: 42,
		Amount: 100.00,
		Status: "SUCCESS", // Already SUCCESS
	}

	mock.ExpectBegin()

	// SELECT FOR UPDATE returns SUCCESS
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM payments WHERE id = $1 FOR UPDATE")).
		WithArgs(pay.ID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("SUCCESS"))

	mock.ExpectRollback()

	updated, err := repo.ProcessWebhookTx(context.Background(), pay, "cs_test_123", "evt_123", "checkout.session.completed", "SUCCESS")
	if err != nil {
		t.Fatalf("expected no error on already processed payment, got: %v", err)
	}
	if updated {
		t.Errorf("expected updated to be false since payment was already processed")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestProcessWebhookTx_WalletUpdateFail_Rollback(t *testing.T) {
	// 3. Database transaction rollback เมื่อ wallet update fail
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	repo := payment.NewRepository(db)

	pay := &payment.Payment{
		ID:     1,
		UserID: 42,
		Amount: 100.00,
		Status: "PENDING",
	}

	mock.ExpectBegin()

	// 1. SELECT FOR UPDATE to lock row
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM payments WHERE id = $1 FOR UPDATE")).
		WithArgs(pay.ID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PENDING"))

	// 2. Insert processed event
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO processed_events (id, event_type) VALUES ($1, $2)")).
		WithArgs("evt_123", "checkout.session.completed").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 3. Update payment status
	mock.ExpectExec(regexp.QuoteMeta("UPDATE payments SET status = 'SUCCESS', paid_at = $1, gateway_reference = $2 WHERE id = $3")).
		WithArgs(sqlmock.AnyArg(), "cs_test_123", pay.ID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 4. Get wallet ID
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM wallets WHERE user_id = $1")).
		WithArgs(pay.UserID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))

	// 5. Insert transaction
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO transactions (wallet_id, type, amount, status, reference_id) VALUES ($1, 'DEPOSIT', $2, 'SUCCESS', $3)")).
		WithArgs(int64(10), pay.Amount, payment.FormatID(pay.ID)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 6. Update wallet balance fails!
	mock.ExpectExec(regexp.QuoteMeta("UPDATE wallets SET balance = balance + $1, updated_at = NOW() WHERE user_id = $2")).
		WithArgs(pay.Amount, pay.UserID).
		WillReturnError(sql.ErrConnDone) // Mock db error

	mock.ExpectRollback()

	updated, err := repo.ProcessWebhookTx(context.Background(), pay, "cs_test_123", "evt_123", "checkout.session.completed", "SUCCESS")
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("expected sql.ErrConnDone, got: %v", err)
	}
	if updated {
		t.Errorf("expected updated to be false on transaction failure")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestProcessWebhookTxEventInsertFailureIsRetryable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := payment.NewRepository(db)
	pay := &payment.Payment{ID: 1, UserID: 42, Amount: 100, Status: "PENDING"}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM payments WHERE id = $1 FOR UPDATE")).WithArgs(pay.ID).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("PENDING"))
	failure := errors.New("database unavailable")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO processed_events (id, event_type) VALUES ($1, $2)")).WithArgs("evt_failure", "checkout.session.completed").WillReturnError(failure)
	mock.ExpectRollback()
	updated, err := repo.ProcessWebhookTx(context.Background(), pay, "cs_test", "evt_failure", "checkout.session.completed", "SUCCESS")
	if updated || !errors.Is(err, failure) {
		t.Fatalf("database error swallowed: updated=%v err=%v", updated, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
