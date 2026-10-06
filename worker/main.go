package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/2m-topup/backend/pkg/cache"
	"github.com/2m-topup/backend/pkg/config"
	"github.com/2m-topup/backend/pkg/database"
	"github.com/2m-topup/backend/pkg/payment"
	"github.com/2m-topup/backend/pkg/queue"
	"github.com/2m-topup/worker/processor"
)

func main() {
	log.Println("[Worker] Starting payment worker...")
	cfg := config.Load()

	// 1. Connect to PostgreSQL
	db, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[Worker] Failed to connect to postgres: %v", err)
	}
	defer db.Close()

	// 2. Connect to Redis
	redisClient := database.NewRedisClient(cfg.RedisURL)
	defer redisClient.Close()

	// Verify Redis connection on startup gracefully
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Printf("[Worker] Warning: Redis is not reachable at startup: %v. Continuing as connections are lazy...", err)
	} else {
		log.Println("[Worker] Redis connection verified successfully")
	}

	redisCache := cache.NewRedisCache(redisClient)
	paymentQueue := queue.NewRedisQueue(redisClient)
	paymentRepo := payment.NewRepository(db)
	proc := processor.NewPaymentProcessor(paymentRepo, redisCache)

	// Context for graceful loop termination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	shutdownChan := make(chan struct{})

	// Handle SIGTERM/SIGINT signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		log.Printf("[Worker] Received signal: %v. Initiating graceful shutdown...", sig)
		cancel()            // Cancel context to break out of the blocking Dequeue loop
		close(shutdownChan) // Tell any active retry timers to fire immediately
	}()

	log.Println("[Worker] Ready to process jobs. Entering main queue listener loop...")

	for {
		if ctx.Err() != nil {
			break
		}

		// Dequeue webhook job (BRPOP with 5s timeout)
		job, err := paymentQueue.DequeueWebhook(ctx, 5*time.Second)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			log.Printf("[Worker] Error dequeuing job: %v", err)
			time.Sleep(1 * time.Second) // Prevent fast error looping
			continue
		}

		// BRPOP returned nil on timeout, loop again
		if job == nil {
			continue
		}

		// Process the job concurrently
		wg.Add(1)
		go func(j queue.WebhookJob) {
			defer wg.Done()

			log.Printf("[Worker] Processing job: payment_id=%s, retry_count=%d", j.PaymentID, j.RetryCount)

			err := proc.ProcessWebhookJob(context.Background(), j)

			var resultStr string
			if err == nil {
				resultStr = "success"
			} else if errors.Is(err, processor.ErrPaymentNotPending) {
				resultStr = "ignored (already processed)"
			} else if errors.Is(err, processor.ErrPaymentNotFound) {
				resultStr = "ignored (payment not found)"
			} else {
				resultStr = "failed: " + err.Error()
			}

			// Log result for every processed job
			log.Printf("[Worker] Job processed: payment_id=%s, result=%s, retry_count=%d", j.PaymentID, resultStr, j.RetryCount)

			// Handle retry strategy for retryable errors
			if err != nil &&
				!errors.Is(err, processor.ErrPaymentNotPending) &&
				!errors.Is(err, processor.ErrPaymentNotFound) {
				handleRetry(paymentQueue, j, err, &wg, shutdownChan)
			}
		}(*job)
	}

	log.Println("[Worker] Stopped listening for new jobs. Waiting for active jobs to complete...")
	wg.Wait()
	log.Println("[Worker] Graceful shutdown complete. Exiting.")
}

func handleRetry(q queue.Queue, job queue.WebhookJob, processErr error, wg *sync.WaitGroup, shutdownChan chan struct{}) {
	job.RetryCount++

	if job.RetryCount > 3 {
		log.Printf("[Worker] Max retries reached for payment %s. Moving to DLQ. Error: %v", job.PaymentID, processErr)
		if err := q.EnqueueDLQ(context.Background(), job); err != nil {
			log.Printf("[Worker] Error enqueuing to DLQ for payment %s: %v", job.PaymentID, err)
		}
		return
	}

	var delay time.Duration
	switch job.RetryCount {
	case 1:
		delay = 5 * time.Second
	case 2:
		delay = 30 * time.Second
	case 3:
		delay = 5 * time.Minute
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		log.Printf("[Worker] Scheduling retry #%d for payment %s in %v", job.RetryCount, job.PaymentID, delay)

		select {
		case <-time.After(delay):
			if err := q.EnqueueWebhook(context.Background(), job); err != nil {
				log.Printf("[Worker] Failed to re-enqueue job for payment %s: %v", job.PaymentID, err)
			} else {
				log.Printf("[Worker] Re-enqueued job for payment %s (retry #%d)", job.PaymentID, job.RetryCount)
			}
		case <-shutdownChan:
			log.Printf("[Worker] Shutdown triggered. Re-enqueueing payment %s immediately without waiting.", job.PaymentID)
			if err := q.EnqueueWebhook(context.Background(), job); err != nil {
				log.Printf("[Worker] Failed to re-enqueue job for payment %s on shutdown: %v", job.PaymentID, err)
			}
		}
	}()
}
