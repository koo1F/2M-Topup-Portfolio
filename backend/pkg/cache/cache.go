// Package cache defines the Cache interface and supporting errors used by
// application services to abstract over different caching backends.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrMiss is returned by Get when the key does not exist in the cache.
var ErrMiss = errors.New("cache: key not found")

// Cache is a simple key-value cache interface.
// Implementations must be safe for concurrent use.
type Cache interface {
	// Get retrieves the value for key. Returns ErrMiss when not found.
	Get(ctx context.Context, key string) (string, error)
	// Set stores value under key with the given TTL.
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	// Delete removes the key from the cache.
	Delete(ctx context.Context, key string) error
	// SetNX sets key=value only if the key does not already exist (NX).
	// Returns true if the key was set, false if it already existed.
	// Used to implement distributed locks.
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
}
