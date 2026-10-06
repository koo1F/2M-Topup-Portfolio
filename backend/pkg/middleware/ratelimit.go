package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter performs sliding-window rate limiting backed by Redis sorted sets.
type Limiter struct {
	client *redis.Client
}

// NewLimiter creates a Limiter that uses the given Redis client.
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{client: client}
}

// RateLimit returns middleware that enforces max requests per window for the
// given limitType and identifier extracted from each request.
//
// Redis key pattern: "ratelimit:{type}:{identifier}"
// Algorithm: sliding window via ZREMRANGEBYSCORE + ZADD + ZCARD.
// On exceed → 429 Too Many Requests with Retry-After header (seconds).
func (l *Limiter) RateLimit(limitType string, max int, window time.Duration, identifier func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := identifier(r)
			key := fmt.Sprintf("ratelimit:%s:%s", limitType, id)

			allowed, retryAfter, err := l.allow(r, key, max, window)
			if err != nil {
				// Fail open: Redis unavailability must not block legitimate traffic.
				log.Printf("[ratelimit] Redis error for key %s — failing open: %v", key, err)
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeRateLimitError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// allow implements the sliding-window check. Returns allowed, retryAfterSeconds, error.
func (l *Limiter) allow(r *http.Request, key string, max int, window time.Duration) (bool, int, error) {
	ctx := r.Context()
	now := time.Now()
	cutoff := float64(now.Add(-window).UnixNano())

	// 1. Remove entries outside the window
	if err := l.client.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%f", cutoff)).Err(); err != nil {
		return false, 0, err
	}

	// 2. Count current entries
	count, err := l.client.ZCard(ctx, key).Result()
	if err != nil {
		return false, 0, err
	}

	// 3. Reject if at or over limit
	if count >= int64(max) {
		retryAfter, err := l.computeRetryAfter(ctx, key, window, now)
		if err != nil {
			return false, 0, err
		}
		return false, retryAfter, nil
	}

	// 4. Record this request
	member := fmt.Sprintf("%d", now.UnixNano())
	score := float64(now.UnixNano())
	pipe := l.client.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: member})
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, 0, err
	}

	return true, 0, nil
}

// computeRetryAfter returns seconds until the oldest entry in the window expires.
func (l *Limiter) computeRetryAfter(ctx context.Context, key string, window time.Duration, now time.Time) (int, error) {
	entries, err := l.client.ZRangeWithScores(ctx, key, 0, 0).Result()
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return int(window.Seconds()), nil
	}

	oldest := time.Unix(0, int64(entries[0].Score))
	remaining := window - now.Sub(oldest)
	secs := int(remaining.Seconds()) + 1 // round up
	if secs < 1 {
		secs = 1
	}
	return secs, nil
}

// IPIdentifier extracts the client IP for rate-limit keys.
func IPIdentifier(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// UserIdentifier extracts the authenticated user ID for rate-limit keys.
// Must run after Auth middleware.
func UserIdentifier(r *http.Request) string {
	id, ok := UserIDFromContext(r.Context())
	if !ok {
		return "anonymous"
	}
	return strconv.FormatInt(id, 10)
}

func writeRateLimitError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
