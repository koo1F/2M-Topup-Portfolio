package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/2m-topup/backend/pkg/cache"
	"github.com/2m-topup/backend/pkg/queue"
	"github.com/stripe/stripe-go/v78"
	checkoutsession "github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/paymentintent"
	"github.com/stripe/stripe-go/v78/webhook"
)

const (
	paymentTTL = 15 * time.Minute
	lockTTL    = 30 * time.Second
)

// Sentinel errors — handlers map these to HTTP status codes.
var (
	ErrInvalidAmount         = errors.New("amount must be between 10.00 and 9999999999999.99 THB with at most two decimal places")
	ErrMissingIdempKey       = errors.New("Idempotency-Key header is required")
	ErrInvalidSignature      = errors.New("invalid webhook signature")
	ErrPaymentNotFound       = errors.New("payment not found")
	ErrPaymentNotPending     = errors.New("payment already processed")
	ErrLockNotAcquired       = errors.New("payment is already being processed")
	ErrIdempotencyConflict   = errors.New("idempotency key was already used with a different request")
	ErrInvalidMethod         = errors.New("method must be card or promptpay")
	ErrInvalidIdempotencyKey = errors.New("idempotency key must contain 1 to 100 characters")
	ErrMockWebhookDisabled   = errors.New("mock webhooks are disabled")
	ErrMissingUserEmail      = errors.New("user email is required for payment")
)

// Service is the interface for payment business logic.
type Service interface {
	CreatePayment(ctx context.Context, userID int64, req CreatePaymentRequest, idempotencyKey string) (*CreatePaymentResponse, bool, error)
	EnqueueWebhook(ctx context.Context, body []byte, stripeSig string, mockSig string) error
	ProcessWebhookJob(ctx context.Context, job queue.WebhookJob) error
	GetPaymentStatus(ctx context.Context, userID int64, paymentIDStr string) (*PaymentStatusResponse, error)
}

type service struct {
	repo              Repository
	cache             cache.Cache
	queue             queue.Queue
	webhookSecret     string
	stripeSuccessURL  string
	stripeCancelURL   string
	allowMockWebhooks bool
}

// NewService creates a payment Service.
func NewService(repo Repository, c cache.Cache, q queue.Queue, webhookSecret, successURL, cancelURL string, allowMock ...bool) Service {
	return &service{
		repo:              repo,
		cache:             c,
		queue:             q,
		webhookSecret:     webhookSecret,
		stripeSuccessURL:  successURL,
		stripeCancelURL:   cancelURL,
		allowMockWebhooks: len(allowMock) > 0 && allowMock[0],
	}
}

// ─── 6.1 Create Payment ────────────────────────────────────────────────────

func (s *service) CreatePayment(ctx context.Context, userID int64, req CreatePaymentRequest, idempotencyKey string) (*CreatePaymentResponse, bool, error) {
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount < 10.00 || req.Amount > 9999999999999.99 || req.Amount != math.Round(req.Amount*100)/100 {
		return nil, false, ErrInvalidAmount
	}
	if len(idempotencyKey) == 0 || len(idempotencyKey) > 100 {
		return nil, false, ErrInvalidIdempotencyKey
	}
	method := strings.ToLower(req.Method)
	if method == "" {
		method = "promptpay"
	}
	if method != "card" && method != "promptpay" {
		return nil, false, ErrInvalidMethod
	}
	cacheKey := fmt.Sprintf("idempotency:payment:%d:%s", userID, idempotencyKey)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var entry idempotencyEntry
		if json.Unmarshal([]byte(cached), &entry) == nil && entry.Response != nil {
			if entry.Amount != req.Amount || entry.Method != method {
				return nil, false, ErrIdempotencyConflict
			}
			return entry.Response, false, nil
		}
	}

	expiresAt := time.Now().Add(paymentTTL)
	p, err := s.repo.CreatePayment(ctx, userID, req.Amount, "", idempotencyKey, method, expiresAt)
	if err != nil {
		if errors.Is(err, ErrDuplicateIdempotencyKey) {
			// Get existing payment from database
			p, err = s.repo.GetPaymentByIdempotencyKey(ctx, userID, idempotencyKey)
			if err != nil || p == nil {
				log.Printf("[Payment Service] Fetch existing payment by idempotency key failed | key=%s | error=%v", idempotencyKey, err)
				return nil, false, fmt.Errorf("fetch duplicate payment: %w", err)
			}
			if p.UserID != userID || p.Amount != req.Amount || p.Method != method {
				return nil, false, ErrIdempotencyConflict
			}
			paymentIDStr := FormatID(p.ID)
			var redirectURL string
			var qrPayload string
			if p.GatewayReference != "" {
				if strings.HasPrefix(p.GatewayReference, "cs_") {
					// Card (Checkout Session)
					sess, err := checkoutsession.Get(p.GatewayReference, nil)
					if err != nil {
						log.Printf("[Payment Service] Reconstruct checkout session from Stripe failed | gateway_ref=%s | error=%v", p.GatewayReference, err)
						return nil, false, fmt.Errorf("retrieve stripe checkout session: %w", err)
					}
					redirectURL = sess.URL
				} else if strings.HasPrefix(p.GatewayReference, "pi_") {
					// PromptPay (PaymentIntent)
					pi, err := paymentintent.Get(p.GatewayReference, nil)
					if err != nil {
						log.Printf("[Payment Service] Reconstruct payment intent from Stripe failed | gateway_ref=%s | error=%v", p.GatewayReference, err)
						return nil, false, fmt.Errorf("retrieve stripe payment intent: %w", err)
					}
					if pi.NextAction != nil && pi.NextAction.PromptPayDisplayQRCode != nil {
						qrPayload = pi.NextAction.PromptPayDisplayQRCode.Data
					}
				}
			}
			resp := &CreatePaymentResponse{
				PaymentID:   paymentIDStr,
				Amount:      p.Amount,
				QRPayload:   qrPayload,
				RedirectURL: redirectURL,
				ExpiresAt:   p.ExpiresAt,
				Status:      p.Status,
			}
			return resp, false, nil
		}
		log.Printf("[Payment Service] Create payment in DB failed | step=database | user_id=%d | amount=%.2f | error=%v", userID, req.Amount, err)
		return nil, false, fmt.Errorf("create payment in DB: %w", err)
	}

	paymentIDStr := FormatID(p.ID)

	// Fetch user email for Stripe
	email, err := s.repo.GetUserEmail(ctx, userID)
	if err != nil {
		log.Printf("[Payment Service] Fetch user email failed | step=database | user_id=%d | amount=%.2f | payment_id=%s | error=%v", userID, req.Amount, paymentIDStr, err)
		_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", err.Error())
		return nil, true, fmt.Errorf("fetch user email: %w", err)
	}

	emailExists := (email != "")
	log.Printf("[Payment Service] User email lookup completed | step=stripe | user_id=%d | payment_id=%s | email_exists=%t", userID, paymentIDStr, emailExists)

	if !emailExists {
		_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", ErrMissingUserEmail.Error())
		return nil, true, ErrMissingUserEmail
	}

	var gatewayRef string
	var qrPayload string
	var redirectURL string

	switch method {
	case "card":
		// Create Stripe Checkout Session
		params := &stripe.CheckoutSessionParams{
			CustomerEmail:      stripe.String(email),
			PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
			Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
						Currency: stripe.String("thb"),
						ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
							Name: stripe.String("Top-up Wallet"),
						},
						UnitAmount: stripe.Int64(int64(math.Round(req.Amount * 100))), // satang
					},
					Quantity: stripe.Int64(1),
				},
			},
			SuccessURL: stripe.String(s.stripeSuccessURL + "?payment_id=" + paymentIDStr),
			CancelURL:  stripe.String(s.stripeCancelURL + "?payment_id=" + paymentIDStr),
			Metadata: map[string]string{
				"payment_id": paymentIDStr,
				"user_id":    strconv.FormatInt(userID, 10),
			},
		}
		params.Context = ctx
		params.SetIdempotencyKey(fmt.Sprintf("topup:%d:%s", userID, idempotencyKey))
		sess, err := checkoutsession.New(params)
		if err != nil {
			_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", err.Error())
			log.Printf("[Payment] step=stripe_create user_id=%d payment_id=%s status=failed error=%v", userID, paymentIDStr, err)

			resp := &CreatePaymentResponse{
				PaymentID: paymentIDStr,
				Amount:    p.Amount,
				ExpiresAt: p.ExpiresAt,
				Status:    "FAILED",
			}
			return resp, true, nil
		}
		gatewayRef = sess.ID
		redirectURL = sess.URL

	case "promptpay":
		// Create and Confirm Stripe PaymentIntent
		params := &stripe.PaymentIntentParams{
			Amount:             stripe.Int64(int64(math.Round(req.Amount * 100))), // satang
			Currency:           stripe.String("thb"),
			PaymentMethodTypes: stripe.StringSlice([]string{"promptpay"}),
			Confirm:            stripe.Bool(true),
			PaymentMethodData: &stripe.PaymentIntentPaymentMethodDataParams{
				Type: stripe.String("promptpay"),
				BillingDetails: &stripe.PaymentIntentPaymentMethodDataBillingDetailsParams{
					Email: stripe.String(email),
				},
			},
			ReceiptEmail: stripe.String(email),
			Metadata: map[string]string{
				"payment_id": paymentIDStr,
				"user_id":    strconv.FormatInt(userID, 10),
			},
		}
		params.Context = ctx
		params.SetIdempotencyKey(fmt.Sprintf("topup:%d:%s", userID, idempotencyKey))
		pi, err := paymentintent.New(params)
		if err != nil {
			_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", err.Error())
			log.Printf("[Payment] step=stripe_create user_id=%d payment_id=%s status=failed error=%v", userID, paymentIDStr, err)

			resp := &CreatePaymentResponse{
				PaymentID: paymentIDStr,
				Amount:    p.Amount,
				ExpiresAt: p.ExpiresAt,
				Status:    "FAILED",
			}
			return resp, true, nil
		}
		gatewayRef = pi.ID

		// Validate PromptPay response details
		if pi.Status != "requires_action" || pi.NextAction == nil || pi.NextAction.PromptPayDisplayQRCode == nil || pi.NextAction.PromptPayDisplayQRCode.Data == "" {
			errVal := fmt.Errorf("stripe payment intent status is not requires_action or QR display payload is invalid/empty")
			_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", errVal.Error())
			log.Printf("[Payment] step=stripe_create user_id=%d payment_id=%s status=failed error=%v", userID, paymentIDStr, errVal)

			resp := &CreatePaymentResponse{
				PaymentID: paymentIDStr,
				Amount:    p.Amount,
				ExpiresAt: p.ExpiresAt,
				Status:    "FAILED",
			}
			return resp, true, nil
		}
		qrPayload = pi.NextAction.PromptPayDisplayQRCode.Data

	default:
		errVal := fmt.Errorf("unsupported payment method: %s", req.Method)
		_ = s.repo.UpdateStatusWithFailure(ctx, p.ID, "FAILED", errVal.Error())
		log.Printf("[Payment] step=stripe_create user_id=%d payment_id=%s status=failed error=%v", userID, paymentIDStr, errVal)

		resp := &CreatePaymentResponse{
			PaymentID: paymentIDStr,
			Amount:    p.Amount,
			ExpiresAt: p.ExpiresAt,
			Status:    "FAILED",
		}
		return resp, true, nil
	}

	// Update gateway reference in DB
	if err := s.repo.UpdateGatewayReference(ctx, p.ID, gatewayRef); err != nil {
		log.Printf("[Payment Service] Update gateway reference in DB failed | step=database | user_id=%d | amount=%.2f | payment_id=%s | error=%v", userID, req.Amount, paymentIDStr, err)
		return nil, true, fmt.Errorf("update gateway reference in DB: %w", err)
	}

	resp := &CreatePaymentResponse{
		PaymentID:   paymentIDStr,
		Amount:      p.Amount,
		ExpiresAt:   p.ExpiresAt,
		Status:      "PENDING",
		QRPayload:   qrPayload,
		RedirectURL: redirectURL,
	}

	// Cache response until payment expires so duplicate keys get the same response
	if data, err := json.Marshal(idempotencyEntry{Amount: req.Amount, Method: method, Response: resp}); err == nil {
		ttl := time.Until(expiresAt)
		_ = s.cache.Set(ctx, cacheKey, string(data), ttl)
	}

	return resp, true, nil
}

// ─── 6.2 Enqueue Webhook (HTTP handler — verify HMAC, queue, return 200) ───

func (s *service) EnqueueWebhook(ctx context.Context, body []byte, stripeSig string, mockSig string) error {
	if stripeSig != "" {
		evt, err := webhook.ConstructEventWithOptions(body, stripeSig, s.webhookSecret, webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		})
		if err != nil {
			secretFormat := "empty"
			if len(s.webhookSecret) > 0 {
				prefix := ""
				if len(s.webhookSecret) >= 6 {
					prefix = s.webhookSecret[:6]
				} else {
					prefix = s.webhookSecret
				}
				secretFormat = fmt.Sprintf("prefix=%s... len=%d", prefix, len(s.webhookSecret))
			}
			log.Printf("[Payment Webhook] Stripe signature verification failed | error=%v | sig_length=%d | webhook_secret=%s", err, len(stripeSig), secretFormat)
			return ErrInvalidSignature
		}

		// Double-spend check using Redis (processed:event:<event_id> TTL 24h)
		eventKey := fmt.Sprintf("processed:event:%s", evt.ID)
		acquired, err := s.cache.SetNX(ctx, eventKey, "1", 24*time.Hour)
		if err != nil {
			return fmt.Errorf("idempotency check error: %w", err)
		}
		if !acquired {
			// Already processed or processing, return nil (idempotent ignore)
			return nil
		}

		var paymentID string
		var status string
		var gatewayRef string

		switch evt.Type {
		case "checkout.session.completed":
			var session stripe.CheckoutSession
			if err := json.Unmarshal(evt.Data.Raw, &session); err != nil {
				return fmt.Errorf("unmarshal checkout session: %w", err)
			}
			paymentID = session.Metadata["payment_id"]
			status = "SUCCESS"
			gatewayRef = session.ID

		case "checkout.session.expired":
			var session stripe.CheckoutSession
			if err := json.Unmarshal(evt.Data.Raw, &session); err != nil {
				return fmt.Errorf("unmarshal checkout session: %w", err)
			}
			paymentID = session.Metadata["payment_id"]
			status = "EXPIRED"
			gatewayRef = session.ID

		case "payment_intent.succeeded":
			var pi stripe.PaymentIntent
			if err := json.Unmarshal(evt.Data.Raw, &pi); err != nil {
				return fmt.Errorf("unmarshal payment intent: %w", err)
			}
			paymentID = pi.Metadata["payment_id"]
			status = "SUCCESS"
			gatewayRef = pi.ID

		default:
			// Unhandled event type, ignore (return nil so Stripe doesn't retry)
			return nil
		}

		if paymentID == "" {
			return fmt.Errorf("missing payment_id in event metadata")
		}

		job := queue.WebhookJob{
			PaymentID:        paymentID,
			Status:           status,
			GatewayReference: gatewayRef,
			EventID:          evt.ID,
			EventType:        string(evt.Type),
			ReceivedAt:       time.Now(),
		}
		if err := s.queue.EnqueueWebhook(ctx, job); err != nil {
			_ = s.cache.Delete(ctx, eventKey)
			return fmt.Errorf("enqueue webhook: %w", err)
		}
		return nil
	}

	// Mock HMAC is explicitly opt-in for local tests.
	if !s.allowMockWebhooks {
		return ErrMockWebhookDisabled
	}
	if !verifyHMAC(body, mockSig, s.webhookSecret) {
		return ErrInvalidSignature
	}

	var req WebhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return err
	}

	job := queue.WebhookJob{
		PaymentID:        req.PaymentID,
		Status:           req.Status,
		GatewayReference: req.GatewayReference,
		EventID:          "",
		EventType:        "",
		ReceivedAt:       time.Now(),
	}
	if err := s.queue.EnqueueWebhook(ctx, job); err != nil {
		return fmt.Errorf("enqueue webhook: %w", err)
	}
	return nil
}

// ─── 6.2b Process Webhook Job (Worker — DB transaction + cache invalidation) ─

func (s *service) ProcessWebhookJob(ctx context.Context, job queue.WebhookJob) error {
	paymentID, err := parsePaymentID(job.PaymentID)
	if err != nil {
		return ErrPaymentNotFound
	}

	p, err := s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return ErrPaymentNotFound
	}

	if p.Status != "PENDING" {
		return ErrPaymentNotPending
	}

	lockKey := fmt.Sprintf("lock:payment:%d", p.ID)
	acquired, err := s.cache.SetNX(ctx, lockKey, "1", lockTTL)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return ErrLockNotAcquired
	}
	defer s.cache.Delete(ctx, lockKey) //nolint:errcheck

	p, err = s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("re-check payment: %w", err)
	}
	if p == nil || p.Status != "PENDING" {
		return ErrPaymentNotPending
	}

	updated, err := s.repo.ProcessWebhookTx(ctx, p, job.GatewayReference, "", "", "SUCCESS")
	if err != nil {
		return fmt.Errorf("process webhook tx: %w", err)
	}
	if !updated {
		return ErrPaymentNotPending
	}

	balanceKey := fmt.Sprintf("wallet:balance:%d", p.UserID)
	_ = s.cache.Delete(ctx, balanceKey)

	statusKey := fmt.Sprintf("payment:status:%s", FormatID(p.ID))
	_ = s.cache.Delete(ctx, statusKey)

	return nil
}

// ─── 6.3 Get Payment Status ─────────────────────────────────────────────────

func (s *service) GetPaymentStatus(ctx context.Context, userID int64, paymentIDStr string) (*PaymentStatusResponse, error) {
	paymentID, err := parsePaymentID(paymentIDStr)
	if err != nil {
		return nil, ErrPaymentNotFound
	}

	// Authorize against the database before reading any cached response.
	p, err := s.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}
	if p == nil || p.UserID != userID {
		return nil, ErrPaymentNotFound
	}
	cacheKey := fmt.Sprintf("payment:status:%s", FormatID(paymentID))
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var resp PaymentStatusResponse
		if json.Unmarshal([]byte(cached), &resp) == nil {
			return &resp, nil
		}
	}

	resp := &PaymentStatusResponse{
		PaymentID: FormatID(p.ID),
		Status:    p.Status,
		Amount:    p.Amount,
		PaidAt:    p.PaidAt,
	}

	if data, err := json.Marshal(resp); err == nil {
		var ttl time.Duration
		if p.Status == "PENDING" {
			ttl = 5 * time.Second
		} else {
			ttl = 24 * time.Hour
		}
		_ = s.cache.Set(ctx, cacheKey, string(data), ttl)
	}

	return resp, nil
}

type idempotencyEntry struct {
	Amount   float64                `json:"amount"`
	Method   string                 `json:"method"`
	Response *CreatePaymentResponse `json:"response"`
}

// ─── Private helpers ─────────────────────────────────────────────────────────

// verifyHMAC checks that signature == "sha256=<hex(HMAC-SHA256(body, secret))>"
func verifyHMAC(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	// Use hmac.Equal to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expected))
}

// parsePaymentID accepts "PAY000001" or "1" and returns the numeric ID.
func parsePaymentID(s string) (int64, error) {
	num := strings.TrimPrefix(s, "PAY")
	id, err := strconv.ParseInt(num, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid payment id: %s", s)
	}
	return id, nil
}

// mockQRPayload generates a realistic-looking (but non-functional) QR payload.
func mockQRPayload(paymentID string, amount float64) string {
	return fmt.Sprintf("00020101021153037645802TH5916MINI PAYMENT6007BANGKOK6222081801%s540%.2f5303764630412AB",
		paymentID, amount)
}
