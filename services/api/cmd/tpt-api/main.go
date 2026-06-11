package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/tpt-online-video/services/api/internal/config"
	"github.com/tpt-online-video/services/api/internal/database"
	httpapi "github.com/tpt-online-video/services/api/internal/http"
	"github.com/tpt-online-video/packages/storage"
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

	srv := httpapi.NewServer(logger, db, redisClient, store, cfg.BaseURL)
	if err := srv.EnsureQueueGroup(ctx); err != nil {
		logger.Error("ensure queue group", "error", err)
		os.Exit(1)
	}

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