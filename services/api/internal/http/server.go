package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/shared"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/handlers"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

type Server struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	redis   *redis.Client
	storage storage.Provider
	queue   *media.Queue
	baseURL string
}

func NewServer(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, baseURL string) *Server {
	return &Server{
		logger:  logger,
		db:      db,
		redis:   redis,
		storage: store,
		queue:   media.NewQueue(redis, "transcode:queue", "transcode-workers", "api"),
		baseURL: baseURL,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recoverer(s.logger))
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Rate limiter (global default)
	rateLimiter := middleware.NewRateLimiter(
		s.redis,
		middleware.DefaultRateLimiterConfig(),
		middleware.IPKeyFunc,
		s.logger,
	)

	// Public routes (no auth required)
	r.Group(func(r chi.Router) {
		r.Use(rateLimiter.Middleware)

		// Health
		r.Get("/healthz", s.healthz)
		r.Get("/readyz", s.readyz)
		r.Get("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
			middleware.WriteOK(w, map[string]string{"message": "pong"})
		})

		// Public video listing
		videoHandler := handlers.NewVideoHandler(s.logger, s.db, s.redis, s.storage, s.baseURL)
		r.Get("/api/v1/videos", videoHandler.ListVideos)
		r.Get("/api/v1/videos/{videoID}", videoHandler.GetVideo)
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		authMiddleware := middleware.NewAuthMiddleware(
			middleware.AuthConfig{
				JWTSecret:     "jwt-secret", // TODO: read from config
				JWTAccessTTL:  15 * time.Minute,
				JWTRefreshTTL: 168 * time.Hour,
			},
			s.logger,
			"/healthz", "/readyz", "/api/v1/ping", "/api/v1/videos",
		)
		r.Use(authMiddleware.Middleware)
		r.Use(rateLimiter.Middleware)

		// Upload routes (upload requires auth)
		uploadHandler := handlers.NewUploadHandler(s.logger, s.db, s.redis, s.storage, s.queue, s.baseURL)
		r.Post("/api/v1/upload", uploadHandler.CreateSession)
		r.Post("/api/v1/upload/{sessionID}/chunk", uploadHandler.UploadChunk)
		r.Post("/api/v1/upload/{sessionID}/complete", uploadHandler.CompleteUpload)

		// Admin routes
		adminMiddleware := middleware.NewAdminMiddleware(s.logger, s.redis)
		r.Group(func(r chi.Router) {
			r.Use(adminMiddleware.AdminAuditLog)
			r.Use(adminMiddleware.RequireAdmin)
			r.Use(adminMiddleware.AdminRateLimiter().Middleware)

			r.Get("/api/v1/admin/health", s.adminHealth)
		})
	})

	// Not found handler
	r.NotFound(middleware.NotFoundHandler().ServeHTTP)
	r.MethodNotAllowed(middleware.MethodNotAllowedHandler().ServeHTTP)

	return r
}

func (s *Server) EnsureQueueGroup(ctx context.Context) error {
	return s.queue.EnsureGroup(ctx)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	middleware.WriteOK(w, shared.Healthy(map[string]string{
		"api":     "ok",
		"storage": s.storage.Name(),
	}))
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	if err := s.db.Ping(ctx); err != nil {
		checks["postgres"] = err.Error()
	} else {
		checks["postgres"] = "ok"
	}

	if err := s.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = err.Error()
	} else {
		checks["redis"] = "ok"
	}

	if err := s.storage.Health(ctx); err != nil {
		checks["storage"] = err.Error()
	} else {
		checks["storage"] = "ok"
	}

	status := http.StatusOK
	body := shared.Healthy(checks)
	for _, value := range checks {
		if value != "ok" {
			status = http.StatusServiceUnavailable
			body = shared.Unhealthy(checks)
			break
		}
	}

	middleware.WriteOK(w, body)
	_ = status // status is handled by WriteOK
}

func (s *Server) adminHealth(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	middleware.WriteOK(w, map[string]interface{}{
		"status":  "ok",
		"admin":   true,
		"user_id": userID,
	})
}
