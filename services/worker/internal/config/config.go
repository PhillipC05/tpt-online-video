package config

import (
	"fmt"
	"os"
	"time"

	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/storage"
)

type Config struct {
	AppEnv      string
	WorkerName  string
	RedisAddr   string
	RedisPass   string
	Storage     storage.Config
	Concurrency int
	WorkDir     string
	MetricsAddr string

	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	Scaler media.ScalerConfig
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:      getenv("APP_ENV", "development"),
		WorkerName:  getenv("WORKER_NAME", "tpt-worker"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getenv("REDIS_PASSWORD", ""),
		Concurrency: getenvInt("WORKER_CONCURRENCY", 1),
		WorkDir:     getenv("WORKER_WORK_DIR", "./data/worker"),
		MetricsAddr: getenv("METRICS_ADDR", ":9091"),

		PostgresHost:     getenv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getenv("POSTGRES_PORT", "5432"),
		PostgresDB:       getenv("POSTGRES_DB", "tpt"),
		PostgresUser:     getenv("POSTGRES_USER", "tpt"),
		PostgresPassword: getenv("POSTGRES_PASSWORD", "tpt"),

		Scaler: media.ScalerConfig{
			MinWorkers:          getenvInt("SCALER_MIN_WORKERS", 1),
			MaxWorkers:          getenvInt("SCALER_MAX_WORKERS", 8),
			ScaleUpQueueDepth:   int64(getenvInt("SCALER_SCALE_UP_QUEUE_DEPTH", 5)),
			ScaleDownQueueDepth: int64(getenvInt("SCALER_SCALE_DOWN_QUEUE_DEPTH", 1)),
			ScaleUpBusyPct:      getenvFloat("SCALER_SCALE_UP_BUSY_PCT", 0.80),
			ScaleDownBusyPct:    getenvFloat("SCALER_SCALE_DOWN_BUSY_PCT", 0.30),
			EvalInterval:        getenvDuration("SCALER_EVAL_INTERVAL", 15*time.Second),
			CooldownUp:          getenvDuration("SCALER_COOLDOWN_UP", 30*time.Second),
			CooldownDown:        getenvDuration("SCALER_COOLDOWN_DOWN", 60*time.Second),
		},

		Storage: storage.Config{
			Provider:          getenv("STORAGE_PROVIDER", "local"),
			LocalRoot:         getenv("LOCAL_STORAGE_ROOT", ""),
			S3Endpoint:        getenv("S3_ENDPOINT", ""),
			S3Bucket:          getenv("S3_BUCKET", "tpt-media"),
			S3Region:          getenv("S3_REGION", "us-east-1"),
			S3AccessKeyID:     getenv("S3_ACCESS_KEY_ID", ""),
			S3SecretAccessKey: getenv("S3_SECRET_ACCESS_KEY", ""),
			S3UsePathStyle:    getenv("S3_USE_PATH_STYLE", "true") != "false",
		},
	}
	if cfg.Concurrency < 1 {
		return Config{}, fmt.Errorf("WORKER_CONCURRENCY must be >= 1")
	}
	return cfg, nil
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUser, c.PostgresPassword, c.PostgresHost, c.PostgresPort, c.PostgresDB)
}

func (c Config) RedisPassword() string {
	return c.RedisPass
}

func (c Config) RedisAddress() string {
	return c.RedisAddr
}

func (c Config) ShutdownTimeout() time.Duration {
	return 10 * time.Second
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if value := os.Getenv(key); value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	if value := os.Getenv(key); value != "" {
		var parsed float64
		if _, err := fmt.Sscanf(value, "%f", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return fallback
}