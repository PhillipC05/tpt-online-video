package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/tpt-online-video/packages/storage"
)

type Config struct {
	AppEnv      string
	Host        string
	Port        string
	BaseURL     string
	CORSOrigins []string

	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	RedisAddr     string
	RedisPassword string

	Storage storage.Config

	JWTSecret        string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	FrontendBaseURL  string
	MediaStreamBaseURL string

	// Live streaming
	MediaMTXHLSBaseURL    string
	MediaMTXWebRTCBaseURL string
	RTMPBaseURL           string
	LiveHookSecret        string
	MediaMTXHLSDirectory  string // local FS path where MediaMTX writes HLS segments

	// Email settings
	EmailProvider    string
	EmailFromName    string
	EmailFromAddress string
	SMTPHost         string
	SMTPPort         int
	SMTPUsername     string
	SMTPPassword     string
	SMTPTLS          bool

	// Admin seed account
	AdminEmail        string
	AdminPassword     string
	AdminDisplayName  string
}

func Load() (Config, error) {
	accessTTL, err := parseDurationEnv("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := parseDurationEnv("JWT_REFRESH_TTL", 168*time.Hour)
	if err != nil {
		return Config{}, err
	}

	smtpPort, _ := strconv.Atoi(getenv("SMTP_PORT", "1025"))

	cfg := Config{
		AppEnv:      getenv("APP_ENV", "development"),
		Host:        getenv("APP_HOST", "0.0.0.0"),
		Port:        getenv("APP_PORT", "8080"),
		BaseURL:     getenv("APP_BASE_URL", "http://localhost:8080"),
		CORSOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),

		PostgresHost:     getenv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getenv("POSTGRES_PORT", "5432"),
		PostgresDB:       getenv("POSTGRES_DB", "tpt"),
		PostgresUser:     getenv("POSTGRES_USER", "tpt"),
		PostgresPassword: getenv("POSTGRES_PASSWORD", "tpt"),

		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),

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

		JWTSecret:        getenv("JWT_SECRET", "change-me-in-development"),
		JWTAccessTTL:     accessTTL,
		JWTRefreshTTL:    refreshTTL,
		FrontendBaseURL:  getenv("FRONTEND_BASE_URL", "http://localhost:5173"),
		MediaStreamBaseURL: getenv("MEDIA_STREAM_BASE_URL", "http://localhost:8080"),

		MediaMTXHLSBaseURL:    getenv("MEDIAMTX_HLS_BASE_URL", "http://localhost:8888"),
		MediaMTXWebRTCBaseURL: getenv("MEDIAMTX_WEBRTC_BASE_URL", "http://localhost:8889"),
		RTMPBaseURL:           getenv("RTMP_BASE_URL", "rtmp://localhost:1935"),
		LiveHookSecret:        getenv("LIVE_HOOK_SECRET", "changeme-live-hook-secret"),
		MediaMTXHLSDirectory:  getenv("MEDIAMTX_HLS_DIRECTORY", "/var/mediamtx/hls"),

		EmailProvider:    getenv("EMAIL_PROVIDER", "log"),
		EmailFromName:    getenv("EMAIL_FROM_NAME", "TPT Online Video"),
		EmailFromAddress: getenv("EMAIL_FROM_ADDRESS", "noreply@tpt.local"),
		SMTPHost:         getenv("SMTP_HOST", "localhost"),
		SMTPPort:         smtpPort,
		SMTPUsername:     getenv("SMTP_USERNAME", ""),
		SMTPPassword:     getenv("SMTP_PASSWORD", ""),
		SMTPTLS:          getenv("SMTP_TLS", "false") == "true",

		AdminEmail:        getenv("ADMIN_EMAIL", ""),
		AdminPassword:     getenv("ADMIN_PASSWORD", ""),
		AdminDisplayName:  getenv("ADMIN_DISPLAY_NAME", "Admin"),
	}

	if cfg.JWTSecret == "change-me-in-development" && cfg.AppEnv == "production" {
		return Config{}, fmt.Errorf("JWT_SECRET must be changed in production")
	}

	return cfg, nil
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
	)
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}