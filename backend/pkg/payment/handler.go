package payment

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/2m-topup/backend/pkg/middleware"
)

// Handler handles HTTP requests for payment endpoints.
type Handler struct {
	svc Service
}

// NewHandler creates a new payment Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// CreatePayment handles POST /api/payment/create
//
//	201 Created  — payment created (or returned from idempotency cache)
//	400          — missing Idempotency-Key or invalid amount
//	401          — no/bad JWT
//	500          — unexpected error
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idempKey := r.Header.Get("Idempotency-Key")
	if idempKey == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, created, err := h.svc.CreatePayment(r.Context(), userID, req, idempKey)
	if err != nil {
		if errors.Is(err, ErrInvalidAmount) || errors.Is(err, ErrMissingUserEmail) || errors.Is(err, ErrInvalidMethod) || errors.Is(err, ErrInvalidIdempotencyKey) {
			log.Printf("[Payment API] Validation error: %v | user_id=%d | amount=%.2f", err, userID, req.Amount)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if errors.Is(err, ErrIdempotencyConflict) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}

		// Structured error logging for HTTP 500
		errMsg := err.Error()
		step := "unexpected"
		if strings.Contains(errMsg, "create payment in DB") {
			step = "database"
		} else if strings.Contains(errMsg, "stripe") {
			step = "stripe"
		} else if strings.Contains(errMsg, "unsupported payment method") {
			step = "validation"
		} else if strings.Contains(errMsg, "idempotency") {
			step = "idempotency"
		}

		log.Printf("[Payment API] HTTP 500 Internal Server Error | step=%s | user_id=%d | amount=%.2f | error=%s",
			step, userID, req.Amount, errMsg)

		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	statusCode := http.StatusCreated
	if !created {
		statusCode = http.StatusOK
	}
	writeJSON(w, statusCode, resp)
}

// Webhook handles POST /api/webhooks/stripe
//
//	200 OK  — job enqueued (HMAC verified)
//	401     — invalid HMAC signature
//	500     — unexpected error
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read body")
		return
	}

	stripeSig := r.Header.Get("Stripe-Signature")
	mockSig := r.Header.Get("X-Gateway-Signature")

	if err := h.svc.EnqueueWebhook(r.Context(), body, stripeSig, mockSig); err != nil {
		if errors.Is(err, ErrInvalidSignature) || errors.Is(err, ErrMockWebhookDisabled) {
			writeError(w, http.StatusUnauthorized, "invalid signature")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// GetStatus handles GET /api/payment/{id}/status
//
//	200 OK  → {"payment_id":"PAY000001","status":"SUCCESS","amount":100.00,"paid_at":"..."}
//	401     → no/bad JWT
//	404     → payment not found
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	paymentIDStr := chi.URLParam(r, "id")

	resp, err := h.svc.GetPaymentStatus(r.Context(), userID, paymentIDStr)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			writeError(w, http.StatusNotFound, "payment not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
