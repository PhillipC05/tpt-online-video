package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/worker/internal/config"
	"github.com/tpt-online-video/services/worker/internal/processor"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load worker configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddress(),
		Password: cfg.RedisPassword(),
	})
	defer redisClient.Close()

	store, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		logger.Error("initialize storage", "error", err)
		os.Exit(1)
	}

	db, err := connectDB(ctx, cfg.PostgresDSN())
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("redis unavailable", "error", err)
		os.Exit(1)
	}
	if err := store.Health(ctx); err != nil {
		logger.Error("storage unavailable", "error", err)
		os.Exit(1)
	}

	// Ensure work directory exists
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		logger.Error("create work directory", "error", err)
		os.Exit(1)
	}

	queue := media.NewQueue(redisClient, "transcode:queue", "transcode-workers", cfg.WorkerName)
	if err := queue.EnsureGroup(ctx); err != nil {
		logger.Error("ensure queue group", "error", err)
		os.Exit(1)
	}

	scaler := media.NewScalingController(queue, cfg.Scaler, logger)
	proc := processor.New(logger, db, redisClient, store, queue, cfg.WorkDir).
		WithScaler(scaler)

	// Expose Prometheus-compatible metrics on a dedicated port.
	metricsAddr := cfg.MetricsAddr
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", proc.Metrics().Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	go func() {
		logger.Info("metrics server listening", "addr", metricsAddr)
		if err := http.ListenAndServe(metricsAddr, mux); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	logger.Info("worker started",
		"name", cfg.WorkerName,
		"concurrency", cfg.Concurrency,
		"scaler_min", cfg.Scaler.MinWorkers,
		"scaler_max", cfg.Scaler.MaxWorkers,
		"storage", store.Name(),
		"work_dir", cfg.WorkDir,
		"metrics_addr", metricsAddr,
	)

	proc.Run(ctx, cfg.Concurrency)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer cancel()
	db.Close()
	if err := redisClient.Close(); err != nil {
		logger.Warn("redis close warning", "error", err)
	}
	if shutdownCtx.Err() == context.DeadlineExceeded {
		logger.Warn("worker shutdown timed out")
	}

	logger.Info("worker stopped")
}

func connectDB(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = 5
	poolCfg.MinConns = 1
	return pgxpool.NewWithConfig(ctx, poolCfg)
}