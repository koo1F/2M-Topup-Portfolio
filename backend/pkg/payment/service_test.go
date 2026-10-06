package payment_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v78"

	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
)

// mockRepository implements payment.Repository for testing.
type mockRepository struct {
	createPaymentFn        func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error)
	getPaymentByIDFn       func(ctx context.Context, id int64) (*payment.Payment, error)
	getPaymentByIdempKeyFn func(ctx context.Context, userID int64, key string) (*payment.Payment, error)
	updateGatewayRefFn     func(ctx context.Context, id int64, gatewayRef string) error
	updateStatusFn         func(ctx context.Context, id int64, status string) error
	updateStatusWithFailFn func(ctx context.Context, id int64, status string, failureReason string) error
	getUserEmailFn         func(ctx context.Context, userID int64) (string, error)
	processWebhookTxFn     func(ctx context.Context, pay *payment.Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (bool, error)
}

func (m *mockRepository) CreatePayment(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
	if m.createPaymentFn != nil {
		return m.createPaymentFn(ctx, userID, amount, gatewayRef, idempotencyKey, method, expiresAt)
	}
	return nil, nil
}

func (m *mockRepository) GetUserEmail(ctx context.Context, userID int64) (string, error) {
	if m.getUserEmailFn != nil {
		return m.getUserEmailFn(ctx, userID)
	}
	return "test@example.com", nil
}

func (m *mockRepository) GetPaymentByID(ctx context.Context, id int64) (*payment.Payment, error) {
	if m.getPaymentByIDFn != nil {
		return m.getPaymentByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRepository) GetPaymentByIdempotencyKey(ctx context.Context, userID int64, key string) (*payment.Payment, error) {
	if m.getPaymentByIdempKeyFn != nil {
		return m.getPaymentByIdempKeyFn(ctx, userID, key)
	}
	return nil, nil
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

func (m *mockRepository) ProcessWebhookTx(ctx context.Context, pay *payment.Payment, gatewayRef string, eventID string, eventType string, targetStatus string) (bool, error) {
	if m.processWebhookTxFn != nil {
		return m.processWebhookTxFn(ctx, pay, gatewayRef, eventID, eventType, targetStatus)
	}
	return false, nil
}

// mockCache implements cache.Cache for testing.
type mockCache struct {
	getFn    func(ctx context.Context, key string) (string, error)
	setFn    func(ctx context.Context, key string, value string, ttl time.Duration) error
	deleteFn func(ctx context.Context, key string) error
	setNXFn  func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}

func (m *mockCache) Get(ctx context.Context, key string) (string, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	return "", nil
}

func (m *mockCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, key)
	}
	return nil
}

func (m *mockCache) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if m.setNXFn != nil {
		return m.setNXFn(ctx, key, value, ttl)
	}
	return false, nil
}

// mockQueue implements queue.Queue for testing.
type mockQueue struct {
	enqueueWebhookFn func(ctx context.Context, job queue.WebhookJob) error
}

func (m *mockQueue) EnqueueWebhook(ctx context.Context, job queue.WebhookJob) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, job)
	}
	return nil
}

func (m *mockQueue) DequeueWebhook(ctx context.Context, timeout time.Duration) (*queue.WebhookJob, error) {
	return nil, nil
}

func (m *mockQueue) EnqueueDLQ(ctx context.Context, job queue.WebhookJob) error {
	return nil
}

// mockStripeBackend mocks stripe.Backend to intercept API calls.
type mockStripeBackend struct {
	stripe.Backend
	callFn func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error
}

func (m *mockStripeBackend) Call(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	return m.callFn(method, path, key, params, v)
}

func TestService_CreatePayment_Card_Success(t *testing.T) {
	// Initialize custom stripe backend mock
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			if path == "/v1/checkout/sessions" {
				sess := v.(*stripe.CheckoutSession)
				sess.ID = "cs_test_12345"
				sess.URL = "https://checkout.stripe.com/pay/cs_test_12345"
				return nil
			}
			return errors.New("unsupported path")
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	// Mock repo calls
	createdPayment := &payment.Payment{
		ID:        1001,
		UserID:    42,
		Amount:    150.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	var updateGatewayRefCalled bool
	var updateGatewayRefVal string

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return createdPayment, nil
		},
		updateGatewayRefFn: func(ctx context.Context, id int64, gatewayRef string) error {
			if id != 1001 {
				t.Errorf("expected payment id 1001, got %d", id)
			}
			updateGatewayRefCalled = true
			updateGatewayRefVal = gatewayRef
			return nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss")
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 150.00,
		Method: "card",
	}

	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-card-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created to be true")
	}

	if resp.PaymentID != "PAY001001" {
		t.Errorf("expected PaymentID PAY001001, got %s", resp.PaymentID)
	}
	if resp.RedirectURL != "https://checkout.stripe.com/pay/cs_test_12345" {
		t.Errorf("expected RedirectURL, got %s", resp.RedirectURL)
	}
	if !updateGatewayRefCalled || updateGatewayRefVal != "cs_test_12345" {
		t.Errorf("expected database update for gateway reference with value cs_test_12345")
	}
}

func TestService_CreatePayment_PromptPay_Success(t *testing.T) {
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			if path == "/v1/payment_intents" {
				pi := v.(*stripe.PaymentIntent)
				pi.ID = "pi_test_12345"
				pi.Status = "requires_action"
				pi.NextAction = &stripe.PaymentIntentNextAction{
					Type: "promptpay_display_qr_code",
					PromptPayDisplayQRCode: &stripe.PaymentIntentNextActionPromptPayDisplayQRCode{
						Data: "0002010102115303764...",
					},
				}
				return nil
			}
			return errors.New("unsupported path")
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	createdPayment := &payment.Payment{
		ID:        1002,
		UserID:    42,
		Amount:    200.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	var updateGatewayRefCalled bool
	var updateGatewayRefVal string

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return createdPayment, nil
		},
		updateGatewayRefFn: func(ctx context.Context, id int64, gatewayRef string) error {
			updateGatewayRefCalled = true
			updateGatewayRefVal = gatewayRef
			return nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss")
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 200.00,
		Method: "promptpay",
	}

	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-pp-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created to be true")
	}

	if resp.PaymentID != "PAY001002" {
		t.Errorf("expected PaymentID PAY001002, got %s", resp.PaymentID)
	}
	if resp.QRPayload != "0002010102115303764..." {
		t.Errorf("expected QRPayload, got %s", resp.QRPayload)
	}
	if !updateGatewayRefCalled || updateGatewayRefVal != "pi_test_12345" {
		t.Errorf("expected database update for gateway reference with value pi_test_12345")
	}
}

func TestService_CreatePayment_StripeFailure_UpdatesToFailed(t *testing.T) {
	// 1. Stripe API failure handling: หลังสร้าง payment record แล้ว ถ้า Stripe creation fail: ต้อง update status เป็น FAILED
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			return &stripe.Error{
				Code:           "api_error",
				Msg:            "Stripe is down",
				HTTPStatusCode: http.StatusInternalServerError,
			}
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	createdPayment := &payment.Payment{
		ID:        1003,
		UserID:    42,
		Amount:    250.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	var updateStatusWithFailCalled bool
	var updateStatusWithFailVal string
	var updateStatusWithFailReason string

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return createdPayment, nil
		},
		updateStatusWithFailFn: func(ctx context.Context, id int64, status string, failureReason string) error {
			if id != 1003 {
				t.Errorf("expected payment id 1003, got %d", id)
			}
			updateStatusWithFailCalled = true
			updateStatusWithFailVal = status
			updateStatusWithFailReason = failureReason
			return nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss")
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 250.00,
		Method: "card",
	}

	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-fail-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created to be true")
	}
	if resp.Status != "FAILED" {
		t.Errorf("expected status to be FAILED, got %s", resp.Status)
	}

	if !updateStatusWithFailCalled || updateStatusWithFailVal != "FAILED" || updateStatusWithFailReason == "" {
		t.Errorf("expected UpdateStatusWithFailure to be called with FAILED and reason, got called=%v, status=%s, reason=%q", updateStatusWithFailCalled, updateStatusWithFailVal, updateStatusWithFailReason)
	}

}

func TestService_CreatePayment_PromptPay_ValidationFailure(t *testing.T) {
	// 2. PromptPay validation: หลังสร้าง PaymentIntent ต้อง verify status, next_action, and QR payload
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			if path == "/v1/payment_intents" {
				pi := v.(*stripe.PaymentIntent)
				pi.ID = "pi_test_12345"
				pi.Status = "requires_payment_method" // INVALID: status is not requires_action
				pi.NextAction = nil                   // INVALID: next action is nil
				return nil
			}
			return errors.New("unsupported path")
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	createdPayment := &payment.Payment{
		ID:        1004,
		UserID:    42,
		Amount:    300.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	var updateStatusWithFailCalled bool
	var updateStatusWithFailVal string
	var updateStatusWithFailReason string

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return createdPayment, nil
		},
		updateStatusWithFailFn: func(ctx context.Context, id int64, status string, failureReason string) error {
			updateStatusWithFailCalled = true
			updateStatusWithFailVal = status
			updateStatusWithFailReason = failureReason
			return nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss")
		},
		setFn: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 300.00,
		Method: "promptpay",
	}

	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-pp-fail-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created to be true")
	}
	if resp.Status != "FAILED" {
		t.Errorf("expected status to be FAILED, got %s", resp.Status)
	}

	if !updateStatusWithFailCalled || updateStatusWithFailVal != "FAILED" || updateStatusWithFailReason == "" {
		t.Errorf("expected UpdateStatusWithFailure to be called with FAILED and reason, got called=%v, status=%s, reason=%q", updateStatusWithFailCalled, updateStatusWithFailVal, updateStatusWithFailReason)
	}
}

func TestService_EnqueueWebhook_Stripe_CheckoutCompleted(t *testing.T) {
	payload := []byte(`{
		"id": "evt_completed_123",
		"object": "event",
		"type": "checkout.session.completed",
		"api_version": "2024-04-10",
		"data": {
			"object": {
				"id": "cs_test_completed_123",
				"object": "checkout.session",
				"metadata": {
					"payment_id": "PAY001001"
				}
			}
		}
	}`)

	secret := "whsec_test"
	timestamp := time.Now().Unix()
	stripeSig := generateStripeSignature(t, payload, secret, timestamp)

	var enqueuedJob *queue.WebhookJob
	q := &mockQueue{
		enqueueWebhookFn: func(ctx context.Context, job queue.WebhookJob) error {
			enqueuedJob = &job
			return nil
		},
	}

	repo := &mockRepository{}

	// Mock cache SetNX to return true (first time)
	var cacheSetNXCalled bool
	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
			if key != "processed:event:evt_completed_123" {
				t.Errorf("expected key processed:event:evt_completed_123, got %s", key)
			}
			cacheSetNXCalled = true
			return true, nil
		},
	}

	svc := payment.NewService(repo, cache, q, secret, "", "", true)

	err := svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cacheSetNXCalled {
		t.Error("expected cache SetNX to be called")
	}

	if enqueuedJob == nil {
		t.Fatal("expected job to be enqueued")
	}

	if enqueuedJob.PaymentID != "PAY001001" {
		t.Errorf("expected PaymentID PAY001001, got %s", enqueuedJob.PaymentID)
	}
	if enqueuedJob.Status != "SUCCESS" {
		t.Errorf("expected Status SUCCESS, got %s", enqueuedJob.Status)
	}
	if enqueuedJob.GatewayReference != "cs_test_completed_123" {
		t.Errorf("expected GatewayReference cs_test_completed_123, got %s", enqueuedJob.GatewayReference)
	}
	if enqueuedJob.EventID != "evt_completed_123" {
		t.Errorf("expected EventID evt_completed_123, got %s", enqueuedJob.EventID)
	}
	if enqueuedJob.EventType != "checkout.session.completed" {
		t.Errorf("expected EventType checkout.session.completed, got %s", enqueuedJob.EventType)
	}
}

func TestService_EnqueueWebhook_Stripe_CheckoutExpired(t *testing.T) {
	payload := []byte(`{
		"id": "evt_expired_123",
		"object": "event",
		"type": "checkout.session.expired",
		"api_version": "2024-04-10",
		"data": {
			"object": {
				"id": "cs_test_expired_123",
				"object": "checkout.session",
				"metadata": {
					"payment_id": "PAY001002"
				}
			}
		}
	}`)

	secret := "whsec_test"
	timestamp := time.Now().Unix()
	stripeSig := generateStripeSignature(t, payload, secret, timestamp)

	var enqueuedJob *queue.WebhookJob
	q := &mockQueue{
		enqueueWebhookFn: func(ctx context.Context, job queue.WebhookJob) error {
			enqueuedJob = &job
			return nil
		},
	}

	repo := &mockRepository{}
	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
			return true, nil
		},
	}

	svc := payment.NewService(repo, cache, q, secret, "", "", true)

	err := svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if enqueuedJob == nil {
		t.Fatal("expected job to be enqueued")
	}

	if enqueuedJob.PaymentID != "PAY001002" {
		t.Errorf("expected PaymentID PAY001002, got %s", enqueuedJob.PaymentID)
	}
	if enqueuedJob.Status != "EXPIRED" {
		t.Errorf("expected Status EXPIRED, got %s", enqueuedJob.Status)
	}
	if enqueuedJob.GatewayReference != "cs_test_expired_123" {
		t.Errorf("expected GatewayReference cs_test_expired_123, got %s", enqueuedJob.GatewayReference)
	}
	if enqueuedJob.EventID != "evt_expired_123" {
		t.Errorf("expected EventID evt_expired_123, got %s", enqueuedJob.EventID)
	}
	if enqueuedJob.EventType != "checkout.session.expired" {
		t.Errorf("expected EventType checkout.session.expired, got %s", enqueuedJob.EventType)
	}
}

func TestService_EnqueueWebhook_Stripe_PaymentIntentSucceeded(t *testing.T) {
	payload := []byte(`{
		"id": "evt_pi_success_123",
		"object": "event",
		"type": "payment_intent.succeeded",
		"api_version": "2024-04-10",
		"data": {
			"object": {
				"id": "pi_test_success_123",
				"object": "payment_intent",
				"metadata": {
					"payment_id": "PAY001003"
				}
			}
		}
	}`)

	secret := "whsec_test"
	timestamp := time.Now().Unix()
	stripeSig := generateStripeSignature(t, payload, secret, timestamp)

	var enqueuedJob *queue.WebhookJob
	q := &mockQueue{
		enqueueWebhookFn: func(ctx context.Context, job queue.WebhookJob) error {
			enqueuedJob = &job
			return nil
		},
	}

	repo := &mockRepository{}
	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
			return true, nil
		},
	}

	svc := payment.NewService(repo, cache, q, secret, "", "", true)

	err := svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if enqueuedJob == nil {
		t.Fatal("expected job to be enqueued")
	}

	if enqueuedJob.PaymentID != "PAY001003" {
		t.Errorf("expected PaymentID PAY001003, got %s", enqueuedJob.PaymentID)
	}
	if enqueuedJob.Status != "SUCCESS" {
		t.Errorf("expected Status SUCCESS, got %s", enqueuedJob.Status)
	}
	if enqueuedJob.GatewayReference != "pi_test_success_123" {
		t.Errorf("expected GatewayReference pi_test_success_123, got %s", enqueuedJob.GatewayReference)
	}
	if enqueuedJob.EventID != "evt_pi_success_123" {
		t.Errorf("expected EventID evt_pi_success_123, got %s", enqueuedJob.EventID)
	}
	if enqueuedJob.EventType != "payment_intent.succeeded" {
		t.Errorf("expected EventType payment_intent.succeeded, got %s", enqueuedJob.EventType)
	}
}

func TestService_EnqueueWebhook_Stripe_IdempotencyRedisCheck(t *testing.T) {
	payload := []byte(`{
		"id": "evt_duplicate_123",
		"object": "event",
		"type": "checkout.session.completed",
		"api_version": "2024-04-10",
		"data": {
			"object": {
				"id": "cs_test_completed_123",
				"object": "checkout.session",
				"metadata": {
					"payment_id": "PAY001001"
				}
			}
		}
	}`)

	secret := "whsec_test"
	timestamp := time.Now().Unix()
	stripeSig := generateStripeSignature(t, payload, secret, timestamp)

	var enqueueCount int
	q := &mockQueue{
		enqueueWebhookFn: func(ctx context.Context, job queue.WebhookJob) error {
			enqueueCount++
			return nil
		},
	}

	repo := &mockRepository{}

	// Mock cache SetNX to simulate:
	// - First call: return true (key set successfully)
	// - Second call: return false (key already exists)
	var cacheCalls int
	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
			cacheCalls++
			if cacheCalls == 1 {
				return true, nil
			}
			return false, nil
		},
	}

	svc := payment.NewService(repo, cache, q, secret, "", "", true)

	// 1st call -> should succeed and enqueue
	err := svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err != nil {
		t.Fatalf("unexpected error on 1st call: %v", err)
	}

	// 2nd call -> should skip (idempotent ignore) and not enqueue
	err = svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err != nil {
		t.Fatalf("unexpected error on 2nd call: %v", err)
	}

	if cacheCalls != 2 {
		t.Errorf("expected cache SetNX to be called 2 times, got %d", cacheCalls)
	}
	if enqueueCount != 1 {
		t.Errorf("expected job to be enqueued only once, got %d", enqueueCount)
	}
}

func TestService_EnqueueWebhook_Stripe_EnqueueFail_DeletesRedisKey(t *testing.T) {
	payload := []byte(`{
		"id": "evt_enqueue_fail_123",
		"object": "event",
		"type": "checkout.session.completed",
		"api_version": "2024-04-10",
		"data": {
			"object": {
				"id": "cs_test_completed_123",
				"object": "checkout.session",
				"metadata": {
					"payment_id": "PAY001001"
				}
			}
		}
	}`)

	secret := "whsec_test"
	timestamp := time.Now().Unix()
	stripeSig := generateStripeSignature(t, payload, secret, timestamp)

	// Mock queue to return enqueue error
	q := &mockQueue{
		enqueueWebhookFn: func(ctx context.Context, job queue.WebhookJob) error {
			return errors.New("queue error")
		},
	}

	repo := &mockRepository{}

	var cacheSetNXCalled bool
	var cacheDeleteCalled bool
	var cacheDeleteKey string

	cache := &mockCache{
		setNXFn: func(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
			cacheSetNXCalled = true
			return true, nil
		},
		deleteFn: func(ctx context.Context, key string) error {
			cacheDeleteCalled = true
			cacheDeleteKey = key
			return nil
		},
	}

	svc := payment.NewService(repo, cache, q, secret, "", "", true)

	err := svc.EnqueueWebhook(context.Background(), payload, stripeSig, "")
	if err == nil {
		t.Fatal("expected error on queue enqueue failure, got nil")
	}

	if !cacheSetNXCalled {
		t.Error("expected cache SetNX to be called")
	}
	if !cacheDeleteCalled || cacheDeleteKey != "processed:event:evt_enqueue_fail_123" {
		t.Errorf("expected cache Delete to be called for key processed:event:evt_enqueue_fail_123, got called=%v, key=%s", cacheDeleteCalled, cacheDeleteKey)
	}
}

func generateStripeSignature(t *testing.T, payload []byte, secret string, timestamp int64) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, expectedSig)
}

func TestService_CreatePayment_MissingEmail(t *testing.T) {
	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return &payment.Payment{
				ID:             99,
				UserID:         userID,
				Amount:         amount,
				Status:         "PENDING",
				IdempotencyKey: idempotencyKey,
				ExpiresAt:      expiresAt,
			}, nil
		},
		getUserEmailFn: func(ctx context.Context, userID int64) (string, error) {
			return "", nil // missing email
		},
	}

	cache := &mockCache{}
	q := &mockQueue{}

	svc := payment.NewService(repo, cache, q, "secret", "http://success", "http://cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 100,
		Method: "promptpay",
	}

	_, _, err := svc.CreatePayment(context.Background(), 42, req, "idem-no-email-001")
	if !errors.Is(err, payment.ErrMissingUserEmail) {
		t.Errorf("expected ErrMissingUserEmail, got %v", err)
	}
}

func TestService_CreatePayment_Idempotency_DatabaseCollision(t *testing.T) {
	// Mock Stripe Backend
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			if path == "/v1/checkout/sessions" {
				sess := v.(*stripe.CheckoutSession)
				sess.ID = "cs_test_9999"
				sess.URL = "https://checkout.stripe.com/pay/cs_test_9999"
				return nil
			}
			if path == "/v1/checkout/sessions/cs_test_9999" && method == "GET" {
				sess := v.(*stripe.CheckoutSession)
				sess.ID = "cs_test_9999"
				sess.URL = "https://checkout.stripe.com/pay/cs_test_9999"
				return nil
			}
			return errors.New("unsupported path")
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	fixedPayment := &payment.Payment{
		ID:               1005,
		UserID:           42,
		Amount:           150.00,
		Status:           "PENDING",
		GatewayReference: "cs_test_9999",
		IdempotencyKey:   "idem-collision-key",
		Method:           "card",
		ExpiresAt:        time.Now().Add(15 * time.Minute),
	}

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			// First call succeeds, second call returns ErrDuplicateIdempotencyKey
			return nil, payment.ErrDuplicateIdempotencyKey
		},
		getPaymentByIdempKeyFn: func(ctx context.Context, userID int64, key string) (*payment.Payment, error) {
			if key != "idem-collision-key" {
				t.Errorf("expected key idem-collision-key, got %s", key)
			}
			return fixedPayment, nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss") // bypass redis cache to force DB unique violation path
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 150.00,
		Method: "card",
	}

	// This call simulates the database constraint violation (cache miss, DB hit)
	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-collision-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created to be false (idempotency hit)")
	}
	if resp.PaymentID != "PAY001005" {
		t.Errorf("expected PAY001005, got %s", resp.PaymentID)
	}
	if resp.RedirectURL != "https://checkout.stripe.com/pay/cs_test_9999" {
		t.Errorf("expected RedirectURL, got %s", resp.RedirectURL)
	}
}

func TestService_CreatePayment_StripeUnavailable_No500(t *testing.T) {
	sb := &mockStripeBackend{
		callFn: func(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
			return errors.New("Stripe API connection failed (network timeout)")
		},
	}
	stripe.SetBackend(stripe.APIBackend, sb)

	createdPayment := &payment.Payment{
		ID:        1006,
		UserID:    42,
		Amount:    150.00,
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	var updateStatusWithFailCalled bool
	var updateStatusWithFailVal string
	var updateStatusWithFailReason string

	repo := &mockRepository{
		createPaymentFn: func(ctx context.Context, userID int64, amount float64, gatewayRef string, idempotencyKey string, method string, expiresAt time.Time) (*payment.Payment, error) {
			return createdPayment, nil
		},
		updateStatusWithFailFn: func(ctx context.Context, id int64, status string, failureReason string) error {
			if id != 1006 {
				t.Errorf("expected payment id 1006, got %d", id)
			}
			updateStatusWithFailCalled = true
			updateStatusWithFailVal = status
			updateStatusWithFailReason = failureReason
			return nil
		},
	}

	cache := &mockCache{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "", errors.New("cache miss")
		},
	}

	svc := payment.NewService(repo, cache, &mockQueue{}, "whsec_test", "http://localhost:3000/success", "http://localhost:3000/cancel", true)

	req := payment.CreatePaymentRequest{
		Amount: 150.00,
		Method: "card",
	}

	resp, created, err := svc.CreatePayment(context.Background(), 42, req, "idem-unavailable-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created to be true")
	}
	if resp.Status != "FAILED" {
		t.Errorf("expected status to be FAILED, got %s", resp.Status)
	}
	if !updateStatusWithFailCalled || updateStatusWithFailVal != "FAILED" || updateStatusWithFailReason == "" {
		t.Errorf("expected UpdateStatusWithFailure to be called with FAILED and reason, got called=%v, status=%s, reason=%q", updateStatusWithFailCalled, updateStatusWithFailVal, updateStatusWithFailReason)
	}
}

func TestService_CreatePayment_InvalidAmount(t *testing.T) {
	repo := &mockRepository{}
	cache := &mockCache{}
	q := &mockQueue{}

	svc := payment.NewService(repo, cache, q, "secret", "http://success", "http://cancel", true)

	// Amount less than 10.00 THB is invalid
	req := payment.CreatePaymentRequest{
		Amount: 9.99,
		Method: "promptpay",
	}

	_, _, err := svc.CreatePayment(context.Background(), 42, req, "idem-invalid-amt")
	if !errors.Is(err, payment.ErrInvalidAmount) {
		t.Errorf("expected ErrInvalidAmount, got %v", err)
	}
}
