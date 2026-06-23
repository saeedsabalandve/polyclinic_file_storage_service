package repository

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// RedisClient wraps the Redis client
type RedisClient struct {
    client *redis.Client
}

// NewRedisClient creates a new Redis client
func NewRedisClient(redisURL string) (*RedisClient, error) {
    opts, err := redis.ParseURL(redisURL)
    if err != nil {
        return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
    }

    client := redis.NewClient(opts)

    // Test connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }

    return &RedisClient{client: client}, nil
}

// Set sets a key-value pair with expiration
func (r *RedisClient) Set(ctx context.Context, key, value string, expiration time.Duration) error {
    return r.client.Set(ctx, key, value, expiration).Err()
}

// Get retrieves a value by key
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
    return r.client.Get(ctx, key).Result()
}

// Del deletes a key
func (r *RedisClient) Del(ctx context.Context, key string) error {
    return r.client.Del(ctx, key).Err()
}

// DelPattern deletes keys matching a pattern
func (r *RedisClient) DelPattern(ctx context.Context, pattern string) error {
    iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
    for iter.Next(ctx) {
        if err := r.client.Del(ctx, iter.Val()).Err(); err != nil {
            return err
        }
    }
    return iter.Err()
}

// Exists checks if a key exists
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
    n, err := r.client.Exists(ctx, key).Result()
    if err != nil {
        return false, err
    }
    return n > 0, nil
}

// Expire sets expiration on a key
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
    return r.client.Expire(ctx, key, expiration).Err()
}

// Incr increments a counter
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
    return r.client.Incr(ctx, key).Result()
}

// HSet sets a hash field
func (r *RedisClient) HSet(ctx context.Context, key, field, value string) error {
    return r.client.HSet(ctx, key, field, value).Err()
}

// HGet gets a hash field
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
    return r.client.HGet(ctx, key, field).Result()
}

// ZAddNX adds a member to a sorted set if it doesn't exist
func (r *RedisClient) ZAddNX(ctx context.Context, key string, score float64, member string) error {
    return r.client.ZAddNX(ctx, key, redis.Z{Score: score, Member: member}).Err()
}

// ZRemRangeByScore removes members by score range
func (r *RedisClient) ZRemRangeByScore(ctx context.Context, key, min, max string) error {
    return r.client.ZRemRangeByScore(ctx, key, min, max).Err()
}

// ZCard returns the number of members in a sorted set
func (r *RedisClient) ZCard(ctx context.Context, key string) (int64, error) {
    return r.client.ZCard(ctx, key).Result()
}

// SetNX sets a key if it doesn't exist
func (r *RedisClient) SetNX(ctx context.Context, key, value string, expiration time.Duration) (bool, error) {
    return r.client.SetNX(ctx, key, value, expiration).Result()
}

// GetClient returns the underlying Redis client
func (r *RedisClient) GetClient() *redis.Client {
    return r.client
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
    return r.client.Close()
}

// IsConnected checks if Redis is connected
func (r *RedisClient) IsConnected(ctx context.Context) bool {
    return r.client.Ping(ctx).Err() == nil
}
