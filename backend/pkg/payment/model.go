package payment

import (
	"fmt"
	"time"
)

// Payment represents a payment record in the database.
type Payment struct {
	ID               int64
	UserID           int64
	Amount           float64
	Status           string // PENDING | SUCCESS | FAILED | EXPIRED
	GatewayReference string
	IdempotencyKey   string
	Method           string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	PaidAt           *time.Time
	FailureReason    string
}

// FormatID formats a numeric payment ID as the canonical string form "PAY000001".
func FormatID(id int64) string {
	return fmt.Sprintf("PAY%06d", id)
}

// CreatePaymentRequest is the body for POST /api/payment/create.
type CreatePaymentRequest struct {
	Amount float64 `json:"amount"`
	Method string  `json:"method"` // "card" or "promptpay"
}

// CreatePaymentResponse is returned after successfully creating a payment.
type CreatePaymentResponse struct {
	PaymentID   string    `json:"payment_id"`
	Amount      float64   `json:"amount"`
	QRPayload   string    `json:"qr_payload,omitempty"`
	RedirectURL string    `json:"redirect_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"`
}

// WebhookRequest is the body of the incoming gateway webhook.
type WebhookRequest struct {
	PaymentID        string `json:"payment_id"`
	Status           string `json:"status"`
	GatewayReference string `json:"gateway_reference"`
}

// PaymentStatusResponse is returned by GET /api/payment/:id/status.
type PaymentStatusResponse struct {
	PaymentID string     `json:"payment_id"`
	Status    string     `json:"status"`
	Amount    float64    `json:"amount"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
}
