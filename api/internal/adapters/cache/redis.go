package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache provides a thin caching layer backed by Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache connects to the Redis instance at the given URL
// (e.g. "redis://localhost:6379/0") and returns a ready-to-use cache.
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("cache: invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves the value for the given key and unmarshals it into dest.
// Returns redis.Nil if the key does not exist.
func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set marshals value as JSON and stores it under key with the given TTL.
// A TTL of 0 means the key does not expire.
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache: failed to marshal value: %w", err)
	}

	return c.client.Set(ctx, key, data, ttl).Err()
}

// Delete removes the given key from the cache.
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists returns true if the key exists in the cache.
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SetNX sets the key only if it does not already exist. Returns true if
// the key was set, false if it already existed. Used for idempotent
// operations like token blacklisting.
func (c *RedisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("cache: failed to marshal value: %w", err)
	}
	return c.client.SetNX(ctx, key, data, ttl).Result()
}

// Close gracefully closes the underlying Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
