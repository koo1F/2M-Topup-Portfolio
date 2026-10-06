package payment_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/2m-topup/backend/pkg/middleware"
	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
)

// --- Mock Service ---

type mockService struct {
	createFn            func(ctx context.Context, userID int64, req payment.CreatePaymentRequest, key string) (*payment.CreatePaymentResponse, bool, error)
	enqueueWebhookFn    func(ctx context.Context, body []byte, stripeSig string, mockSig string) error
	processWebhookJobFn func(ctx context.Context, job queue.WebhookJob) error
	getStatusFn         func(ctx context.Context, userID int64, id string) (*payment.PaymentStatusResponse, error)
}

func (m *mockService) CreatePayment(ctx context.Context, userID int64, req payment.CreatePaymentRequest, key string) (*payment.CreatePaymentResponse, bool, error) {
	return m.createFn(ctx, userID, req, key)
}

func (m *mockService) EnqueueWebhook(ctx context.Context, body []byte, stripeSig string, mockSig string) error {
	if m.enqueueWebhookFn != nil {
		return m.enqueueWebhookFn(ctx, body, stripeSig, mockSig)
	}
	return nil
}

func (m *mockService) ProcessWebhookJob(ctx context.Context, job queue.WebhookJob) error {
	if m.processWebhookJobFn != nil {
		return m.processWebhookJobFn(ctx, job)
	}
	return nil
}

func (m *mockService) GetPaymentStatus(ctx context.Context, userID int64, id string) (*payment.PaymentStatusResponse, error) {
	return m.getStatusFn(ctx, userID, id)
}

// ─── CreatePayment tests ──────────────────────────────────────────────────────

func TestCreatePayment_MissingIdempotencyKey(t *testing.T) {
	h := payment.NewHandler(&mockService{})

	body := mustMarshal(t, map[string]any{"amount": 100.0})
	r := httptest.NewRequest(http.MethodPost, "/api/payment/create", bytes.NewReader(body))
	r = r.WithContext(middleware.WithUserID(r.Context(), 1))
	r.Header.Set("Content-Type", "application/json")
	// No Idempotency-Key header
	w := httptest.NewRecorder()

	h.CreatePayment(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d — body: %s", w.Code, w.Body.String())
	}
}

func TestCreatePayment_Success(t *testing.T) {
	expiresAt := time.Now().Add(15 * time.Minute)
	svc := &mockService{
		createFn: func(_ context.Context, _ int64, req payment.CreatePaymentRequest, key string) (*payment.CreatePaymentResponse, bool, error) {
			return &payment.CreatePaymentResponse{
				PaymentID: "PAY000001",
				Amount:    req.Amount,
				QRPayload: "00020101...",
				ExpiresAt: expiresAt,
				Status:    "PENDING",
			}, true, nil
		},
	}
	h := payment.NewHandler(svc)

	body := mustMarshal(t, map[string]any{"amount": 100.0})
	w, r := newPaymentRequest(t, http.MethodPost, "/api/payment/create", body, 1)
	r.Header.Set("Idempotency-Key", "test-key-001")

	h.CreatePayment(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp payment.CreatePaymentResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if resp.PaymentID != "PAY000001" {
		t.Errorf("expected PaymentID=PAY000001, got %s", resp.PaymentID)
	}
	if resp.Status != "PENDING" {
		t.Errorf("expected status=PENDING, got %s", resp.Status)
	}
}

func TestCreatePayment_Idempotency(t *testing.T) {
	expiresAt := time.Now().Add(15 * time.Minute)
	callCount := 0
	fixedResp := &payment.CreatePaymentResponse{
		PaymentID: "PAY000001",
		Amount:    100.0,
		QRPayload: "mock-qr",
		ExpiresAt: expiresAt,
		Status:    "PENDING",
	}
	svc := &mockService{
		createFn: func(_ context.Context, _ int64, _ payment.CreatePaymentRequest, _ string) (*payment.CreatePaymentResponse, bool, error) {
			callCount++
			isNew := (callCount == 1)
			return fixedResp, isNew, nil
		},
	}
	h := payment.NewHandler(svc)

	sendRequest := func() *httptest.ResponseRecorder {
		body := mustMarshal(t, map[string]any{"amount": 100.0})
		w, r := newPaymentRequest(t, http.MethodPost, "/api/payment/create", body, 1)
		r.Header.Set("Idempotency-Key", "same-key")
		h.CreatePayment(w, r)
		return w
	}

	w1 := sendRequest()
	w2 := sendRequest()

	if w1.Code != http.StatusCreated {
		t.Fatalf("first request should return 201 Created, got %d", w1.Code)
	}
	if w2.Code != http.StatusOK {
		t.Fatalf("second request should return 200 OK, got %d", w2.Code)
	}

	var r1, r2 payment.CreatePaymentResponse
	mustDecode(t, w1.Body.Bytes(), &r1)
	mustDecode(t, w2.Body.Bytes(), &r2)

	if r1.PaymentID != r2.PaymentID {
		t.Errorf("payment_id must be identical: %s != %s", r1.PaymentID, r2.PaymentID)
	}
}

// ─── Webhook tests ────────────────────────────────────────────────────────────

func TestWebhook_InvalidSignature(t *testing.T) {
	svc := &mockService{
		enqueueWebhookFn: func(_ context.Context, _ []byte, _ string, _ string) error {
			return payment.ErrInvalidSignature
		},
	}
	h := payment.NewHandler(svc)

	body := mustMarshal(t, payment.WebhookRequest{PaymentID: "PAY000001", Status: "SUCCESS"})
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Gateway-Signature", "sha256=invalidsig")
	w := httptest.NewRecorder()

	h.Webhook(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// TestWebhook_Enqueued verifies that a valid webhook returns 200 with status=queued.
func TestWebhook_Enqueued(t *testing.T) {
	svc := &mockService{
		enqueueWebhookFn: func(_ context.Context, _ []byte, _ string, _ string) error {
			return nil
		},
	}
	h := payment.NewHandler(svc)

	body := mustMarshal(t, payment.WebhookRequest{PaymentID: "PAY000001", Status: "SUCCESS"})
	r := httptest.NewRequest(http.MethodPost, "/api/webhooks/stripe", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Gateway-Signature", validSignature(t, body, "test-secret"))
	w := httptest.NewRecorder()

	h.Webhook(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	mustDecode(t, w.Body.Bytes(), &resp)
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %s", resp["status"])
	}
}

// ─── GetStatus tests ──────────────────────────────────────────────────────────

func TestGetStatus_Success(t *testing.T) {
	now := time.Now()
	svc := &mockService{
		getStatusFn: func(_ context.Context, userID int64, id string) (*payment.PaymentStatusResponse, error) {
			if userID != 1 {
				t.Fatalf("authenticated user not forwarded: %d", userID)
			}
			return &payment.PaymentStatusResponse{
				PaymentID: "PAY000001",
				Status:    "SUCCESS",
				Amount:    100.0,
				PaidAt:    &now,
			}, nil
		},
	}
	h := payment.NewHandler(svc)

	r := chiRequestWithParam(t, "GET", "/api/payment/{id}/status", "id", "PAY000001", 1)
	w := httptest.NewRecorder()
	h.GetStatus(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp payment.PaymentStatusResponse
	mustDecode(t, w.Body.Bytes(), &resp)

	if resp.Status != "SUCCESS" {
		t.Errorf("expected status=SUCCESS, got %s", resp.Status)
	}
}

func TestGetStatus_NotFound(t *testing.T) {
	svc := &mockService{
		getStatusFn: func(_ context.Context, _ int64, _ string) (*payment.PaymentStatusResponse, error) {
			return nil, payment.ErrPaymentNotFound
		},
	}
	h := payment.NewHandler(svc)

	r := chiRequestWithParam(t, "GET", "/api/payment/{id}/status", "id", "PAY999999", 1)
	w := httptest.NewRecorder()
	h.GetStatus(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newPaymentRequest(t *testing.T, method, path string, body []byte, userID int64) (*httptest.ResponseRecorder, *http.Request) {
	t.Helper()
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))
	return httptest.NewRecorder(), r
}

// chiRequestWithParam creates a request with a chi URL param injected (no real router needed).
func chiRequestWithParam(t *testing.T, method, pattern, paramKey, paramVal string, userID int64) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/api/payment/%s/status", paramVal)
	r := httptest.NewRequest(method, path, nil)
	r = r.WithContext(middleware.WithUserID(r.Context(), userID))

	// Inject chi route context so chi.URLParam works without a real router
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(paramKey, paramVal)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	return r
}

func validSignature(t *testing.T, body []byte, secret string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func mustDecode(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode: %v — raw: %s", err, data)
	}
}

// Ensure mockService satisfies the interface at compile time.
var _ payment.Service = (*mockService)(nil)

// Suppress unused error (errors is used in the file via errors.New indirectly)
var _ = errors.New
