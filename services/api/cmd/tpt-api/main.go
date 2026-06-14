package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tpt-online-video/packages/search"
	"github.com/tpt-online-video/packages/storage"
	svcauth "github.com/tpt-online-video/services/api/internal/auth"
	"github.com/tpt-online-video/services/api/internal/config"
	"github.com/tpt-online-video/services/api/internal/database"
	httpapi "github.com/tpt-online-video/services/api/internal/http"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := database.Connect(ctx, database.Config{DSN: cfg.PostgresDSN()})
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer redisClient.Close()

	store, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		logger.Error("initialize storage", "error", err)
		os.Exit(1)
	}
	searchProvider := search.NewPostgresProvider(db)

	// Seed admin account if configured
	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		seedAdmin(ctx, logger, db, cfg)
	}

	srv := httpapi.NewServer(logger, db, redisClient, store, searchProvider, cfg.BaseURL).
		WithJWTSecret(cfg.JWTSecret, cfg.JWTAccessTTL).
		WithFrontendURL(cfg.FrontendBaseURL).
		WithCORSOrigins(cfg.CORSOrigins).
		WithLiveConfig(cfg.MediaMTXHLSBaseURL, cfg.MediaMTXWebRTCBaseURL, cfg.RTMPBaseURL, cfg.LiveHookSecret, cfg.MediaMTXHLSDirectory)

	if err := srv.EnsureQueueGroup(ctx); err != nil {
		logger.Error("ensure queue group", "error", err)
		os.Exit(1)
	}

	srv.StartDVRCleaner(ctx)

	server := &http.Server{
		Addr:              cfg.Host + ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("api server starting", "addr", server.Addr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", "error", err)
	}

	logger.Info("api server stopped")
}

// seedAdmin creates the admin user on startup if configured.
func seedAdmin(ctx context.Context, logger *slog.Logger, db *pgxpool.Pool, cfg config.Config) {
	repo := svcauth.NewRepository(db)
	hasher := auth.NewPasswordHasher()

	// Check if admin already exists
	existing, err := repo.GetUserByEmail(ctx, cfg.AdminEmail)
	if err != nil {
		logger.Error("check admin user", "error", err)
		return
	}
	if existing != nil {
		logger.Info("admin user already exists", "email", cfg.AdminEmail)
		return
	}

	// Hash password
	passwordHash, err := hasher.Hash(cfg.AdminPassword)
	if err != nil {
		logger.Error("hash admin password", "error", err)
		return
	}

	// Create admin user
	user, err := repo.CreateUser(ctx, cfg.AdminEmail, passwordHash, cfg.AdminDisplayName)
	if err != nil {
		logger.Error("create admin user", "error", err)
		return
	}

	// Assign admin role
	if err := repo.AssignDefaultRole(ctx, user.ID); err != nil {
		logger.Error("assign admin default role", "error", err)
	}

	// Also assign admin role specifically
	_, err = db.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id)
		 SELECT $1, id FROM roles WHERE name = 'admin'
		 ON CONFLICT DO NOTHING`,
		user.ID,
	)
	if err != nil {
		logger.Error("assign admin role", "error", err)
	}

	// Verify email for admin
	if err := repo.VerifyUserEmail(ctx, user.ID); err != nil {
		logger.Error("verify admin email", "error", err)
	}

	logger.Info("admin user created", "email", cfg.AdminEmail, "display_name", cfg.AdminDisplayName)
}
