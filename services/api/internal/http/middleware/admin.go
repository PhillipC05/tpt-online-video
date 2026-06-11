package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// AdminMiddleware provides admin-specific middleware functionality.
type AdminMiddleware struct {
	logger *slog.Logger
	redis  *redis.Client
}

// NewAdminMiddleware creates a new admin middleware.
func NewAdminMiddleware(logger *slog.Logger, redis *redis.Client) *AdminMiddleware {
	return &AdminMiddleware{
		logger: logger,
		redis:  redis,
	}
}

// RequireAdmin returns a middleware that restricts access to admin users only.
func (am *AdminMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := GetUserRole(r)
		if role != "admin" {
			WriteForbidden(w, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireModOrAdmin returns a middleware that allows moderators and admins.
func (am *AdminMiddleware) RequireModOrAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := GetUserRole(r)
		if role != "admin" && role != "moderator" {
			WriteForbidden(w, "moderator or admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AdminAuditLog is a middleware that logs admin actions for audit purposes.
func (am *AdminMiddleware) AdminAuditLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := GetUserID(r)
		role := GetUserRole(r)

		if userID != "" && role == "admin" {
			am.logger.Info("admin action",
				"user_id", userID,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		}

		next.ServeHTTP(w, r)
	})
}

// AdminRateLimiter returns a stricter rate limiter for admin endpoints.
func (am *AdminMiddleware) AdminRateLimiter() *RateLimiter {
	return NewRateLimiter(
		am.redis,
		RateLimiterConfig{
			RequestsPerWindow: 30,
			WindowDuration:    1 * time.Minute,
			BurstSize:         5,
		},
		UserIDKeyFunc,
		am.logger,
	)
}