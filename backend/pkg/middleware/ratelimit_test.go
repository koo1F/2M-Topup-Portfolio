package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/2m-topup/backend/pkg/middleware"
)

func TestRateLimit_ExceedsAfter10Requests(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	limiter := middleware.NewLimiter(client)
	handler := limiter.RateLimit("payment:create", 10, time.Minute, func(_ *http.Request) string {
		return "42"
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	doRequest := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/payment/create", nil)
		r = r.WithContext(middleware.WithUserID(r.Context(), 42))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	for i := 1; i <= 10; i++ {
		w := doRequest()
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	w := doRequest()
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request 11: expected 429, got %d — body: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("request 11: expected Retry-After header")
	}
}

func TestRateLimit_RedisKeyPattern(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	limiter := middleware.NewLimiter(client)
	handler := limiter.RateLimit("login", 5, time.Minute, middleware.IPIdentifier)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.RemoteAddr = "203.0.113.5:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	keys := mr.Keys()
	expected := "ratelimit:login:203.0.113.5"
	found := false
	for _, k := range keys {
		if k == expected {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Redis key %q, got keys: %v", expected, keys)
	}
}

func TestRateLimit_FailOpenOnRedisError(t *testing.T) {
	// Create a client with a closed/non-existent address to force connection errors
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:54321", // Non-existent port
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { _ = client.Close() })

	limiter := middleware.NewLimiter(client)
	handler := limiter.RateLimit("login", 5, time.Minute, func(_ *http.Request) string {
		return "failopen-user"
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Since Redis is down, it should fail-open and return 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK (fail-open), got %d", w.Code)
	}
}
