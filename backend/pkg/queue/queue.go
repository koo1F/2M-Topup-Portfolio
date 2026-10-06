package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// WebhookQueueKey is the Redis list key for incoming payment webhook jobs.
	WebhookQueueKey = "queue:payment:webhook"
	// DLQKey is the dead-letter queue for jobs that failed after MaxRetries.
	DLQKey = "queue:payment:dlq"
	// MaxRetries is the number of processing attempts before a job moves to the DLQ.
	MaxRetries = 3
)

// WebhookJob is the payload enqueued after a webhook passes HMAC verification.
type WebhookJob struct {
	PaymentID        string    `json:"payment_id"`
	Status           string    `json:"status"`
	GatewayReference string    `json:"gateway_reference"`
	EventID          string    `json:"event_id"`
	EventType        string    `json:"event_type"`
	ReceivedAt       time.Time `json:"received_at"`
	RetryCount       int       `json:"retry_count"`
}

// Queue abstracts Redis list operations for webhook processing.
type Queue interface {
	// EnqueueWebhook pushes a job onto the webhook queue (LPUSH).
	EnqueueWebhook(ctx context.Context, job WebhookJob) error
	// DequeueWebhook blocks until a job is available or timeout elapses (BRPOP).
	DequeueWebhook(ctx context.Context, timeout time.Duration) (*WebhookJob, error)
	// EnqueueDLQ pushes a failed job onto the dead-letter queue (LPUSH).
	EnqueueDLQ(ctx context.Context, job WebhookJob) error
}

// RedisQueue is the Redis-backed implementation of Queue.
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue wraps a *redis.Client as a Queue.
func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{client: client}
}

// EnqueueWebhook serialises job and LPUSHes it onto WebhookQueueKey.
func (q *RedisQueue) EnqueueWebhook(ctx context.Context, job WebhookJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, WebhookQueueKey, data).Err()
}

// DequeueWebhook BRPOPs from WebhookQueueKey. Returns nil, nil on timeout.
func (q *RedisQueue) DequeueWebhook(ctx context.Context, timeout time.Duration) (*WebhookJob, error) {
	result, err := q.client.BRPop(ctx, timeout, WebhookQueueKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(result) < 2 {
		return nil, errors.New("queue: unexpected BRPOP result")
	}

	var job WebhookJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// EnqueueDLQ serialises job and LPUSHes it onto DLQKey.
func (q *RedisQueue) EnqueueDLQ(ctx context.Context, job WebhookJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, DLQKey, data).Err()
}
