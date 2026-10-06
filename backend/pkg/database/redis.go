package database

import (
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a go-redis client with explicit connection timeouts.
// Timeouts ensure Railway Redis connectivity issues surface quickly (fail fast)
// rather than blocking for OS-level TCP timeouts.
//
// Address handling:
//   - redis:// or rediss:// → parsed via redis.ParseURL (handles auth + TLS)
//   - plain host:port        → used directly (local dev default)
func NewRedisClient(addr string) *redis.Client {
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		opt, err := redis.ParseURL(addr)
		if err != nil {
			log.Fatalf("failed to parse redis url: %v", err)
		}
		// Apply timeouts after parsing so they override any defaults
		applyTimeouts(opt)
		client := redis.NewClient(opt)
		log.Printf("Redis client ready (URL parsed, address=%s)", opt.Addr)
		return client
	}

	client := redis.NewClient(applyTimeouts(&redis.Options{
		Addr: addr,
	}))
	log.Printf("Redis client ready (addr=%s)", addr)
	return client
}

// applyTimeouts sets conservative connection timeouts and returns the options.
func applyTimeouts(opt *redis.Options) *redis.Options {
	opt.DialTimeout = 3 * time.Second
	opt.ReadTimeout = 3 * time.Second
	opt.WriteTimeout = 3 * time.Second
	opt.PoolTimeout = 4 * time.Second
	return opt
}
