package processor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/2m-topup/backend/pkg/cache"
	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
)

var (
	ErrLockNotAcquired   = errors.New("lock: payment is already being processed")
	ErrPaymentNotFound   = errors.New("payment: not found")
	ErrPaymentNotPending = errors.New("payment: already processed (not pending)")
)

// PaymentProcessor processes incoming payment webhook jobs.
type PaymentProcessor struct {
	repo  payment.Repository
	cache cache.Cache
}

// NewPaymentProcessor creates a new PaymentProcessor.
func NewPaymentProcessor(repo payment.Repository, cache cache.Cache) *PaymentProcessor {
	return &PaymentProcessor{
		repo:  repo,
		cache: cache,
	}
}

// ProcessWebhookJob processes a single webhook job.
// It acquires a distributed lock, checks/re-checks status, updates the database in a transaction,
// invalidates the wallet balance cache, and releases the lock.
func (p *PaymentProcessor) ProcessWebhookJob(ctx context.Context, job queue.WebhookJob) error {
	paymentID, err := parsePaymentID(job.PaymentID)
	if err != nil {
		return ErrPaymentNotFound
	}

	// 1. Initial check (fast fail without lock)
	pay, err := p.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if pay == nil {
		return ErrPaymentNotFound
	}
	if pay.Status != "PENDING" {
		return ErrPaymentNotPending
	}

	// 2. Acquire Lock (TTL 30s)
	lockKey := fmt.Sprintf("lock:payment:%s", job.PaymentID)
	acquired, err := p.cache.SetNX(ctx, lockKey, "1", 30*time.Second)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		return ErrLockNotAcquired
	}
	defer func() {
		if err := p.cache.Delete(ctx, lockKey); err != nil {
			log.Printf("[Worker] Warning: failed to release lock %s: %v", lockKey, err)
		}
	}()

	// 3. Re-check state inside lock
	pay, err = p.repo.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("re-check payment: %w", err)
	}
	if pay == nil || pay.Status != "PENDING" {
		return ErrPaymentNotPending
	}

	// 4. Process Webhook Database Transaction
	updated, err := p.repo.ProcessWebhookTx(ctx, pay, job.GatewayReference, job.EventID, job.EventType, job.Status)
	if err != nil {
		return fmt.Errorf("process webhook tx: %w", err)
	}
	if !updated {
		return ErrPaymentNotPending
	}

	// 5. Invalidate Redis wallet balance cache
	balanceKey := fmt.Sprintf("wallet:balance:%d", pay.UserID)
	if err := p.cache.Delete(ctx, balanceKey); err != nil {
		log.Printf("[Worker] Warning: failed to invalidate balance cache for user %d: %v", pay.UserID, err)
	}

	// Invalidate payment status cache
	statusKey := fmt.Sprintf("payment:status:%s", payment.FormatID(pay.ID))
	if err := p.cache.Delete(ctx, statusKey); err != nil {
		log.Printf("[Worker] Warning: failed to invalidate status cache for payment %d: %v", pay.ID, err)
	}

	return nil
}

// parsePaymentID converts "PAY000001" or "1" to numeric int64.
func parsePaymentID(s string) (int64, error) {
	num := strings.TrimPrefix(s, "PAY")
	id, err := strconv.ParseInt(num, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid payment id format: %s", s)
	}
	return id, nil
}
