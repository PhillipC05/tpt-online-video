package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiterConfig configures the rate limiter.
type RateLimiterConfig struct {
	RequestsPerWindow int
	WindowDuration    time.Duration
	BurstSize         int
}

// DefaultRateLimiterConfig returns a sensible default configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerWindow: 100,
		WindowDuration:    1 * time.Minute,
		BurstSize:         10,
	}
}

// StrictRateLimiterConfig returns a strict configuration for auth endpoints.
func StrictRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerWindow: 10,
		WindowDuration:    1 * time.Minute,
		BurstSize:         3,
	}
}

// KeyFunc extracts a rate limit key from the request (e.g., IP, user ID, API key).
type KeyFunc func(r *http.Request) string

// IPKeyFunc returns the IP address from the request.
func IPKeyFunc(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	idx := strings.LastIndex(r.RemoteAddr, ":")
	if idx == -1 {
		return r.RemoteAddr
	}
	return r.RemoteAddr[:idx]
}

// UserIDKeyFunc returns the user ID from the request context.
func UserIDKeyFunc(r *http.Request) string {
	if userID, ok := r.Context().Value(userIDKey).(string); ok {
		return "user:" + userID
	}
	return ""
}

// RateLimiter is a middleware that rate limits requests using Redis.
type RateLimiter struct {
	client  *redis.Client
	config  RateLimiterConfig
	keyFunc KeyFunc
	logger  *slog.Logger
}

// NewRateLimiter creates a new rate limiter middleware.
func NewRateLimiter(client *redis.Client, config RateLimiterConfig, keyFunc KeyFunc, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		client:  client,
		config:  config,
		keyFunc: keyFunc,
		logger:  logger,
	}
}

// Middleware returns the HTTP middleware handler.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.keyFunc(r)
		if key == "" {
			// Fall back to IP
			key = IPKeyFunc(r)
		}

		allowed, remaining, reset, err := rl.allow(r.Context(), key)
		if err != nil {
			rl.logger.Error("rate limiter error", "error", err, "key", key)
			// On error, allow the request through but log
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.config.RequestsPerWindow))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(reset-time.Now().Unix())))
			WriteTooManyRequests(w, fmt.Sprintf("rate limit exceeded, retry after %d seconds", int(reset-time.Now().Unix())))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// allow checks if a request is allowed under the rate limit using a sliding window
// implemented with Redis sorted sets.
func (rl *RateLimiter) allow(ctx context.Context, key string) (bool, int, int64, error) {
	now := time.Now().UnixMilli()
	windowStart := now - rl.config.WindowDuration.Milliseconds()
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	burstKey := fmt.Sprintf("ratelimit:burst:%s", key)

	// Use a Redis transaction to atomically:
	// 1. Remove old entries outside the window
	// 2. Count entries in the current window
	// 3. Add the current request
	pipe := rl.client.TxPipeline()

	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	countCmd := pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{
		Score:  float64(now),
		Member: now,
	})
	pipe.Expire(ctx, redisKey, rl.config.WindowDuration*2)

	// Burst check: track per-second burst
	burstSecond := now / 1000
	burstWindowStart := burstSecond - 1
	pipe.ZRemRangeByScore(ctx, burstKey, "0", fmt.Sprintf("%d", burstWindowStart))
	burstCountCmd := pipe.ZCard(ctx, burstKey)
	pipe.ZAdd(ctx, burstKey, redis.Z{
		Score:  float64(burstSecond),
		Member: now,
	})
	pipe.Expire(ctx, burstKey, 5*time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, 0, fmt.Errorf("redis exec: %w", err)
	}

	count := countCmd.Val()
	burstCount := burstCountCmd.Val()

	// Check burst limit
	if int(burstCount) > rl.config.BurstSize {
		return false, 0, now/1000 + 1, nil
	}

	// Check window limit
	if int(count) > rl.config.RequestsPerWindow {
		return false, 0, now/1000 + int64(rl.config.WindowDuration.Seconds()), nil
	}

	remaining := rl.config.RequestsPerWindow - int(count)
	if remaining < 0 {
		remaining = 0
	}
	reset := now/1000 + int64(rl.config.WindowDuration.Seconds())

	return true, remaining, reset, nil
}