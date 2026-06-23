package middleware

import (
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/polyclinic/file-storage-service/internal/config"
    "github.com/polyclinic/file-storage-service/internal/repository"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// RateLimiter implements rate limiting middleware
func RateLimiter(redisClient *repository.RedisClient, limitCfg config.RateLimitConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Get client identifier (IP or tenant ID)
            clientID := getClientIdentifier(r)
            
            // Check rate limit
            allowed, retryAfter, err := checkRateLimit(redisClient, clientID, limitCfg)
            if err != nil {
                // Log error but allow request through in case of Redis failure
                next.ServeHTTP(w, r)
                return
            }

            if !allowed {
                w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limitCfg.RequestsPerMinute))
                w.Header().Set("X-RateLimit-Remaining", "0")
                w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))
                w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
                
                utils.RespondWithError(w, http.StatusTooManyRequests, 
                    fmt.Sprintf("Rate limit exceeded. Try again in %d seconds", int(retryAfter.Seconds())))
                return
            }

            // Add rate limit headers
            remaining := getRemainingRequests(redisClient, clientID, limitCfg)
            w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limitCfg.RequestsPerMinute))
            w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
            w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

            next.ServeHTTP(w, r)
        })
    }
}

// getClientIdentifier returns unique identifier for rate limiting
func getClientIdentifier(r *http.Request) string {
    // Use tenant ID if available, otherwise use IP
    if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
        return fmt.Sprintf("tenant:%s", tenantID)
    }
    
    // Use X-Forwarded-For if behind proxy
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return fmt.Sprintf("ip:%s", xff)
    }
    
    return fmt.Sprintf("ip:%s", r.RemoteAddr)
}

// checkRateLimit checks if request is within rate limit
func checkRateLimit(redis *repository.RedisClient, clientID string, cfg config.RateLimitConfig) (bool, time.Duration, error) {
    if redis == nil {
        // No Redis, allow all requests
        return true, 0, nil
    }

    key := fmt.Sprintf("ratelimit:%s", clientID)
    
    // Use sliding window algorithm
    now := time.Now()
    windowStart := now.Add(-time.Minute)
    
    // Remove old entries
    redis.ZRemRangeByScore(key, "0", strconv.FormatInt(windowStart.Unix(), 10))
    
    // Count requests in current window
    count, err := redis.ZCard(key)
    if err != nil {
        return true, 0, err
    }
    
    if count >= int64(cfg.RequestsPerMinute) {
        return false, time.Minute - now.Sub(windowStart), nil
    }
    
    // Add current request
    score := float64(now.UnixNano()) / 1e9
    member := fmt.Sprintf("%d", now.UnixNano())
    err = redis.ZAddNX(key, score, member)
    if err != nil {
        return true, 0, err
    }
    
    // Set expiry on the key
    redis.Expire(key, time.Minute)
    
    return true, 0, nil
}

// getRemainingRequests returns remaining requests count
func getRemainingRequests(redis *repository.RedisClient, clientID string, cfg config.RateLimitConfig) int {
    if redis == nil {
        return cfg.RequestsPerMinute
    }
    
    key := fmt.Sprintf("ratelimit:%s", clientID)
    count, err := redis.ZCard(key)
    if err != nil {
        return cfg.RequestsPerMinute
    }
    
    remaining := cfg.RequestsPerMinute - int(count)
    if remaining < 0 {
        remaining = 0
    }
    
    return remaining
}

// AdaptiveRateLimiter implements dynamic rate limiting based on system load
type AdaptiveRateLimiter struct {
    systemLoadThreshold float64
    rateReductionFactor float64
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(threshold, reductionFactor float64) *AdaptiveRateLimiter {
    return &AdaptiveRateLimiter{
        systemLoadThreshold: threshold,
        rateReductionFactor: reductionFactor,
    }
}

// GetAdjustedLimit calculates rate limit based on system load
func (arl *AdaptiveRateLimiter) GetAdjustedLimit(baseLimit int, currentLoad float64) int {
    if currentLoad > arl.systemLoadThreshold {
        return int(float64(baseLimit) * (1 - arl.rateReductionFactor))
    }
    return baseLimit
}
