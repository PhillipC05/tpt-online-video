module github.com/tpt-online-video/services/api

go 1.22

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.6.0
	github.com/redis/go-redis/v9 v9.6.1
	github.com/tpt-online-video/packages/auth v0.0.0
	github.com/tpt-online-video/packages/media v0.0.0
	github.com/tpt-online-video/packages/moderation v0.0.0
	github.com/tpt-online-video/packages/search v0.0.0
	github.com/tpt-online-video/packages/shared v0.0.0
	github.com/tpt-online-video/packages/storage v0.0.0
	golang.org/x/crypto v0.26.0
)