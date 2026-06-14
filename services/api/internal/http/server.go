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
	"github.com/tpt-online-video/packages/auth"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/search"
	"github.com/tpt-online-video/packages/shared"
	"github.com/tpt-online-video/packages/storage"
	svcauth "github.com/tpt-online-video/services/api/internal/auth"
	"github.com/tpt-online-video/services/api/internal/http/handlers"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
	svclive "github.com/tpt-online-video/services/api/internal/live"
	svcmod "github.com/tpt-online-video/services/api/internal/moderation"
)

type Server struct {
	logger       *slog.Logger
	db           *pgxpool.Pool
	redis        *redis.Client
	storage      storage.Provider
	search       search.Provider
	queue        *media.Queue
	baseURL      string
	jwtSecret    string
	jwtAccessTTL time.Duration
	frontendURL  string
	corsOrigins  []string // allowed CORS origins; defaults to frontendURL

	// Live streaming
	mediamtxHLSBase    string
	mediamtxWebRTCBase string
	mediamtxHLSDir     string
	rtmpBase           string
	liveHookSecret     string

	// Live chat hub (WebSocket room manager)
	chatHub *svclive.ChatHub

	// Metrics
	apiMetrics *middleware.APIMetrics
}

func NewServer(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, searchProvider search.Provider, baseURL string) *Server {
	return &Server{
		logger:             logger,
		db:                 db,
		redis:              redis,
		storage:            store,
		search:             searchProvider,
		queue:              media.NewQueue(redis, "transcode:queue", "transcode-workers", "api"),
		baseURL:            baseURL,
		jwtSecret:          "jwt-secret", // Will be overridden via WithJWTSecret
		jwtAccessTTL:       15 * time.Minute,
		frontendURL:        "http://localhost:5173",
		mediamtxHLSBase:    "http://localhost:8888",
		mediamtxWebRTCBase: "http://localhost:8889",
		rtmpBase:           "rtmp://localhost:1935",
		liveHookSecret:     "changeme-live-hook-secret",
		chatHub:            svclive.NewChatHub(redis, logger),
		apiMetrics:         middleware.NewAPIMetrics(),
	}
}

// WithJWTSecret sets the JWT secret on the server (called after config load).
func (s *Server) WithJWTSecret(secret string, accessTTL time.Duration) *Server {
	s.jwtSecret = secret
	s.jwtAccessTTL = accessTTL
	return s
}

// WithFrontendURL sets the frontend base URL for auth emails.
func (s *Server) WithFrontendURL(url string) *Server {
	s.frontendURL = url
	return s
}

// WithCORSOrigins sets the list of allowed CORS origins.
func (s *Server) WithCORSOrigins(origins []string) *Server {
	s.corsOrigins = origins
	return s
}

// WithLiveConfig sets MediaMTX and RTMP base URLs for live streaming.
func (s *Server) WithLiveConfig(hlsBase, webRTCBase, rtmpBase, hookSecret, hlsDir string) *Server {
	s.mediamtxHLSBase = hlsBase
	s.mediamtxWebRTCBase = webRTCBase
	s.mediamtxHLSDir = hlsDir
	s.rtmpBase = rtmpBase
	s.liveHookSecret = hookSecret
	return s
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recoverer(s.logger))
	r.Use(chimiddleware.Timeout(60 * time.Second))
	r.Use(s.apiMetrics.Middleware)
	r.Use(middleware.RequestLogger(s.logger))
	r.Use(middleware.SecurityHeaders)

	// CORS middleware — origin allowlist; never use wildcard with credentials.
	// State-changing endpoints require an Authorization: Bearer header, so CSRF is
	// not applicable for XHR callers; the cookie fallback path is SameSite=Strict.
	corsOrigins := s.corsOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{s.frontendURL}
	}
	r.Use(middleware.CORSMiddleware(corsOrigins))

	// Rate limiter (global default)
	rateLimiter := middleware.NewRateLimiter(
		s.redis,
		middleware.DefaultRateLimiterConfig(),
		middleware.IPKeyFunc,
		s.logger,
	)

	// Strict rate limiter for auth endpoints
	strictLimiter := middleware.NewRateLimiter(
		s.redis,
		middleware.StrictRateLimiterConfig(),
		middleware.IPKeyFunc,
		s.logger,
	)

	// Auth middleware
	authMW := middleware.NewAuthMiddleware(
		middleware.AuthConfig{
			JWTSecret:     s.jwtSecret,
			JWTAccessTTL:  s.jwtAccessTTL,
			JWTRefreshTTL: 168 * time.Hour,
		},
		s.logger,
		"/healthz", "/readyz", "/api/v1/ping", "/api/v1/videos",
	)

	// Build email config and auth service
	emailCfg := auth.EmailConfig{
		FromName:    "TPT Online Video",
		FromEmail:   "noreply@tpt.local",
		Provider:    "log", // Use SMTP in production
		AppBaseURL:  s.baseURL,
	}

	authSvcCfg := svcauth.ServiceConfig{
		JWTSecret:        s.jwtSecret,
		JWTAccessTTL:     s.jwtAccessTTL,
		JWTRefreshTTL:    168 * time.Hour,
		PasswordResetTTL: 1 * time.Hour,
		FrontendBaseURL:  s.frontendURL,
	}

	emailSender := auth.NewEmailSender(emailCfg)
	authHandler := handlers.NewAuthHandler(s.logger, s.db, emailSender, authSvcCfg, authMW).
		WithAllowedOrigins(corsOrigins)

	videoHandler := handlers.NewVideoHandler(s.logger, s.db, s.redis, s.storage, s.search, s.baseURL)
	searchHandler := handlers.NewSearchHandler(s.logger, s.search)
	commentHandler := handlers.NewCommentHandler(s.logger, s.db)
	profileHandler := handlers.NewProfileHandler(s.logger, s.db, s.storage)

	// Live streaming — wire up repository, service, and handler
	liveRepo := svclive.NewRepository(s.db)
	liveSvcCfg := svclive.ServiceConfig{
		RTMPBaseURL:      s.rtmpBase + "/live",
		HLSBaseURL:       s.mediamtxHLSBase + "/live",
		WebRTCBaseURL:    s.mediamtxWebRTCBase + "/live",
		DVRDefaultWindow: 900,
		HLSDirectory:     s.mediamtxHLSDir,
	}
	liveSvc := svclive.NewService(liveRepo, s.logger, liveSvcCfg)
	liveHandler := handlers.NewLiveHandler(s.logger, s.db, liveSvc, s.liveHookSecret)

	// Live chat — single handler shared across public and authenticated route groups
	chatRepo := svclive.NewChatRepository(s.db)
	chatSvc := svclive.NewChatService(chatRepo, s.chatHub, s.logger)
	chatHandler := handlers.NewChatHandler(s.logger, s.db, chatSvc, liveRepo)

	// Public routes (no auth required)
	r.Group(func(r chi.Router) {
		r.Use(rateLimiter.Middleware)

		// Health
		r.Get("/healthz", s.healthz)
		r.Get("/readyz", s.readyz)
		r.Get("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
			middleware.WriteOK(w, map[string]string{"message": "pong"})
		})

		// Public video listing; GetVideo uses optional auth so private videos
		// can be gated by ownership without requiring auth on public videos.
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/videos", videoHandler.ListVideos)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/videos/{videoID}", videoHandler.GetVideo)
		r.Get("/api/v1/videos/{videoID}/related", videoHandler.RelatedVideos)
		r.Get("/api/v1/videos/{videoID}/comments", commentHandler.ListComments)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/search/autocomplete", searchHandler.Autocomplete)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/search", searchHandler.Search)

		// Public channel/profile routes
		r.Get("/api/v1/channels/{userID}", profileHandler.GetChannel)
		r.Get("/api/v1/channels/{userID}/videos", profileHandler.ListChannelVideos)
		r.Get("/api/v1/channels/{userID}/live", profileHandler.ListChannelLiveStreams)

		// Public live stream metadata (optional auth for owner access)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/live/streams/{streamID}", liveHandler.GetStream)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/live/streams/{streamID}/dvr", liveHandler.GetDVRInfo)
		r.Get("/api/v1/live/streams/live", liveHandler.ListLiveStreams)

		// Live chat — WebSocket and history are public (optional auth)
		r.With(authMW.OptionalAuthMiddleware).Get("/api/v1/live/streams/{streamID}/chat/ws", chatHandler.Connect)
		r.Get("/api/v1/live/streams/{streamID}/chat/messages", chatHandler.ListMessages)

		// MediaMTX internal hooks — no user auth, validated by X-Hook-Secret
		r.Post("/api/v1/live/hooks/auth", liveHandler.HookAuth)
		r.Post("/api/v1/live/hooks/on-publish", liveHandler.HookOnPublish)
		r.Post("/api/v1/live/hooks/on-unpublish", liveHandler.HookOnUnpublish)

		// Auth routes (public - register, login, refresh, forgot/reset password)
		r.Group(func(r chi.Router) {
			r.Use(strictLimiter.Middleware) // Stricter rate limiting for auth
			r.Post("/api/v1/auth/register", authHandler.Register)
			r.Post("/api/v1/auth/login", authHandler.Login)
			r.Post("/api/v1/auth/refresh", authHandler.Refresh)
			r.Post("/api/v1/auth/forgot-password", authHandler.ForgotPassword)
			r.Post("/api/v1/auth/reset-password", authHandler.ResetPassword)
			r.Post("/api/v1/auth/oauth/{provider}", authHandler.OAuthLogin)
		})
	})

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(authMW.Middleware)
		r.Use(rateLimiter.Middleware)

		// Authenticated auth routes
		r.Post("/api/v1/auth/logout", authHandler.Logout)
		r.Post("/api/v1/auth/change-password", authHandler.ChangePassword)
		r.Get("/api/v1/auth/me", authHandler.GetMe)
		r.Get("/api/v1/auth/sessions", authHandler.GetSessions)
		r.Delete("/api/v1/auth/sessions/{sessionID}", authHandler.RevokeSession)

		// Owned video routes (update, delete, signed URL refresh)
		r.Patch("/api/v1/videos/{videoID}", videoHandler.UpdateVideo)
		r.Delete("/api/v1/videos/{videoID}", videoHandler.DeleteVideo)
		r.Get("/api/v1/videos/{videoID}/signed-urls", videoHandler.GetSignedURLs)

		// Video likes
		r.Post("/api/v1/videos/{videoID}/like", commentHandler.LikeVideo)
		r.Delete("/api/v1/videos/{videoID}/like", commentHandler.UnlikeVideo)
		r.Get("/api/v1/videos/{videoID}/like", commentHandler.GetVideoLikeStatus)

		// Comments
		r.Post("/api/v1/videos/{videoID}/comments", commentHandler.CreateComment)
		r.Patch("/api/v1/comments/{commentID}", commentHandler.UpdateComment)
		r.Delete("/api/v1/comments/{commentID}", commentHandler.DeleteComment)
		r.Post("/api/v1/comments/{commentID}/report", commentHandler.ReportComment)

		// Comment likes
		r.Post("/api/v1/comments/{commentID}/like", commentHandler.LikeComment)
		r.Delete("/api/v1/comments/{commentID}/like", commentHandler.UnlikeComment)

		// Profile update and image uploads
		r.Patch("/api/v1/auth/me/profile", profileHandler.UpdateProfile)
		r.Post("/api/v1/auth/me/avatar", profileHandler.UploadAvatar)
		r.Post("/api/v1/auth/me/banner", profileHandler.UploadBanner)

		// Upload routes (upload requires auth)
		uploadHandler := handlers.NewUploadHandler(s.logger, s.db, s.redis, s.storage, s.queue, s.baseURL)
		r.Post("/api/v1/upload", uploadHandler.CreateSession)
		r.Post("/api/v1/upload/{sessionID}/chunk", uploadHandler.UploadChunk)
		r.Post("/api/v1/upload/{sessionID}/complete", uploadHandler.CompleteUpload)
		r.Post("/api/v1/upload/{sessionID}/cancel", uploadHandler.CancelUpload)
		r.Get("/api/v1/upload/sessions", uploadHandler.ListUploadSessions)
		r.Get("/api/v1/upload/{sessionID}", uploadHandler.GetUploadStatus)

		// Live stream management (owner only)
		r.Post("/api/v1/live/streams", liveHandler.CreateStream)
		r.Get("/api/v1/live/streams", liveHandler.ListMyStreams)
		r.Patch("/api/v1/live/streams/{streamID}", liveHandler.UpdateStream)
		r.Delete("/api/v1/live/streams/{streamID}", liveHandler.DeleteStream)
		r.Get("/api/v1/live/streams/{streamID}/urls", liveHandler.GetStreamURLs)

		// Live chat moderation (owner or mod/admin)
		r.Delete("/api/v1/live/streams/{streamID}/chat/messages/{messageID}", chatHandler.DeleteMessage)
		r.Post("/api/v1/live/streams/{streamID}/chat/users/{userID}/timeout", chatHandler.TimeoutUser)
		r.Delete("/api/v1/live/streams/{streamID}/chat/users/{userID}/timeout", chatHandler.RemoveTimeout)
		r.Post("/api/v1/live/streams/{streamID}/chat/users/{userID}/ban", chatHandler.BanUser)
		r.Delete("/api/v1/live/streams/{streamID}/chat/users/{userID}/ban", chatHandler.UnbanUser)
		r.Post("/api/v1/live/streams/{streamID}/chat/lock", chatHandler.LockChat)
		r.Delete("/api/v1/live/streams/{streamID}/chat/lock", chatHandler.UnlockChat)

		// User report routes
		modRepo := svcmod.NewRepository(s.db)
		modSvc := svcmod.NewService(modRepo)
		modHandler := handlers.NewModerationHandler(s.logger, s.db, modSvc)

		// Video report (any authenticated user)
		r.Post("/api/v1/reports", modHandler.CreateReport)
		r.Post("/api/v1/videos/{videoID}/report", modHandler.ReportVideo)
		r.Post("/api/v1/users/{userID}/report", modHandler.ReportUser)
		r.Post("/api/v1/live/streams/{streamID}/report", modHandler.ReportLiveStream)
		r.Post("/api/v1/live/chat/{messageID}/report", modHandler.ReportLiveChatMessage)

		// Appeal submission
		r.Post("/api/v1/reports/{reportID}/appeal", modHandler.SubmitAppeal)

		// Admin/moderation routes
		adminMiddleware := middleware.NewAdminMiddleware(s.logger, s.redis)
		adminHandler := handlers.NewAdminHandler(s.logger, s.db, s.redis, s.queue, s.storage, s.search).
			WithChatHub(s.chatHub)
		r.Group(func(r chi.Router) {
			r.Use(adminMiddleware.AdminAuditLog)
			r.Use(adminMiddleware.RequireModOrAdmin)
			r.Use(adminMiddleware.AdminRateLimiter().Middleware)

			// Dashboard
			r.Get("/api/v1/admin/moderation/stats", modHandler.DashboardStats)

			// Reports
			r.Get("/api/v1/admin/reports", modHandler.ListReports)
			r.Get("/api/v1/admin/reports/{reportID}", modHandler.GetReport)
			r.Post("/api/v1/admin/reports/{reportID}/assign", modHandler.AssignReport)
			r.Post("/api/v1/admin/reports/{reportID}/unassign", modHandler.UnassignReport)
			r.Post("/api/v1/admin/reports/{reportID}/resolve", modHandler.ResolveReport)
			r.Post("/api/v1/admin/reports/{reportID}/dismiss", modHandler.DismissReport)
			r.Put("/api/v1/admin/reports/{reportID}/notes", modHandler.SetAdminNotes)
			r.Post("/api/v1/admin/reports/{reportID}/appeal", modHandler.ResolveAppeal)

			// Moderation actions
			r.Post("/api/v1/admin/moderation/actions", modHandler.ExecuteAction)
			r.Get("/api/v1/admin/moderation/actions", modHandler.ListActions)
			r.Get("/api/v1/admin/moderation/actions/{actionID}", modHandler.GetAction)
			r.Post("/api/v1/admin/moderation/actions/{actionID}/reverse", modHandler.ReverseAction)

			// Audit log
			r.Get("/api/v1/admin/audit-log", modHandler.ListAuditLog)

			// Admin health
			r.Get("/api/v1/admin/health", s.adminHealth)

			// User management (admin only enforced in handler for role/status changes)
			r.Get("/api/v1/admin/users", adminHandler.ListUsers)
			r.Patch("/api/v1/admin/users/{userID}", adminHandler.UpdateUser)

			// Video management
			r.Get("/api/v1/admin/videos", adminHandler.ListVideos)
			r.Patch("/api/v1/admin/videos/{videoID}", adminHandler.UpdateVideo)
			r.Delete("/api/v1/admin/videos/{videoID}", adminHandler.DeleteVideo)

			// Comment management
			r.Get("/api/v1/admin/comments", adminHandler.ListComments)
			r.Patch("/api/v1/admin/comments/{commentID}", adminHandler.UpdateComment)

			// System status, metrics, and settings
			r.Get("/api/v1/admin/system/status", adminHandler.SystemStatus)
			r.Get("/api/v1/admin/system/metrics", s.apiMetrics.Handler())
			r.Get("/api/v1/admin/settings", adminHandler.GetSettings)
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

// StartDVRCleaner launches the DVR segment cleanup background goroutine.
func (s *Server) StartDVRCleaner(ctx context.Context) {
	liveRepo := svclive.NewRepository(s.db)
	cleaner := svclive.NewDVRCleaner(liveRepo, s.logger, s.mediamtxHLSDir, s.mediamtxHLSBase, 5*time.Minute)
	go cleaner.Run(ctx)
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

	if s.search != nil {
		if err := s.search.Health(ctx); err != nil {
			checks["search"] = err.Error()
		} else {
			checks["search"] = "ok"
		}
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