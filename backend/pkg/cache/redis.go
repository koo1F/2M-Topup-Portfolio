package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is the Redis-backed implementation of Cache.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache wraps a *redis.Client as a Cache.
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// Get retrieves a value by key. Returns ErrMiss if the key doesn't exist.
func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrMiss
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Set stores value under key with the given TTL.
func (c *RedisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, key, value, ttl).Err()
}

// Delete removes key from cache. Returns nil if the key didn't exist.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// SetNX implements the Redis SET NX (set-if-not-exists) command.
// Returns true if the key was newly set, false if it already existed.
func (c *RedisCache) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, ttl).Result()
}
