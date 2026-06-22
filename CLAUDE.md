# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Project Is

TPT Online Video is a self-hostable, YouTube-style video platform. It has three separately-deployable Go services (API, Worker, Live helper), a React frontend, and shared Go/TypeScript packages — all managed as a monorepo with Go workspaces and pnpm workspaces.

## Development Commands

### Infrastructure (required first)
```sh
make infra          # Start Postgres, Redis, MinIO, MediaMTX via Docker
make infra-down     # Stop infrastructure
```

### Running Services Locally
```sh
make api            # Run Go API server (port 8080)
make worker         # Run Go transcoding worker
make web            # Run Vite dev server (port 5173, proxies /api to :8080)
```

### Go (services + packages)
```sh
make test-go        # Run all Go tests
make lint-go        # Lint all Go code
go test ./...       # From within a specific service directory
```

### Frontend (apps/web)
```sh
pnpm dev            # Dev server
pnpm build          # Production build
pnpm typecheck      # TypeScript check
pnpm lint           # ESLint
pnpm format         # Prettier
pnpm test           # Vitest (unit tests)
pnpm test:ui        # Vitest with browser UI
```

### Production
```sh
make docker-build   # Build all Docker images
make docker-up-prod # Start full production stack
make lint-all       # Lint everything (Go + frontend)
make clean          # Remove local ./data/ directories
```

### Database Migrations
Migrations live in [migrations/](migrations/) using golang-migrate format. They run automatically on API startup. To add a new migration, create numbered SQL files: `migrations/000X_description.up.sql` and `000X_description.down.sql`.

## Architecture

### Service Topology

```
OBS/Encoder → MediaMTX (RTMP ingest) → HLS/WebRTC output
                                              ↑ DVR sliding window

Browser → React UI (Vite) → Go API (chi) → PostgreSQL (metadata, FTS, audit log)
                                          → Redis (job queue, rate limiting, live chat pub/sub)
                                          → Storage Provider (local / S3 / Wasabi)
                                                ↑
                                           Go Worker (FFmpeg → HLS renditions → storage)
```

### Go Workspace Layout

`go.work` ties together three services and six shared packages:

| Path | Role |
|------|------|
| `services/api/` | HTTP server — auth, uploads, transcoding dispatch, search, moderation, live |
| `services/worker/` | Transcoding daemon — Redis queue consumer, FFmpeg execution, HLS upload |
| `services/live/` | MediaMTX lifecycle helper (minimal; placeholder for future coordination) |
| `packages/auth/` | Argon2id hashing, JWT + opaque refresh token management |
| `packages/media/` | FFmpeg helpers, Redis job queue abstraction, progress reporting |
| `packages/moderation/` | File-type validation, ClamAV scanner interface |
| `packages/search/` | Search provider abstraction (PostgreSQL FTS default; Meilisearch planned) |
| `packages/storage/` | Storage provider abstraction (local, S3-compatible, Wasabi) |
| `packages/shared/` | Health check helpers, shared utilities |

### API Entry Points

- **API main:** [services/api/cmd/tpt-api/main.go](services/api/cmd/tpt-api/main.go) — wires DB, Redis, storage, starts HTTP server and DVR cleaner
- **Worker main:** [services/worker/cmd/tpt-worker/main.go](services/worker/cmd/tpt-worker/main.go) — connects queue, starts transcoding processor, exposes Prometheus metrics
- **HTTP server + router:** [services/api/internal/http/server.go](services/api/internal/http/server.go) — chi router setup, middleware chain (auth, CORS, rate limiting, security headers)
- **Config:** [services/api/internal/config/config.go](services/api/internal/config/config.go) — all config loaded from environment variables only (no config files)

### Key Handler Files

- [services/api/internal/http/handlers/upload.go](services/api/internal/http/handlers/upload.go) — resumable chunked uploads + presigned S3 for large files
- [services/api/internal/http/handlers/video.go](services/api/internal/http/handlers/video.go) — video metadata, playback manifests
- [services/api/internal/http/handlers/live.go](services/api/internal/http/handlers/live.go) — live stream metadata, DVR, WebRTC/WHEP
- [services/api/internal/http/handlers/auth.go](services/api/internal/http/handlers/auth.go) — login, token rotation with reuse detection, password reset
- [services/api/internal/http/handlers/search.go](services/api/internal/http/handlers/search.go) — full-text search with ranking/filters
- [services/api/internal/http/handlers/moderation.go](services/api/internal/http/handlers/moderation.go) — admin moderation workflow

### Frontend

`apps/web/` is a React 18 + TypeScript + Vite app. [apps/web/vite.config.js](apps/web/vite.config.js) proxies `/api` to the local API server. Shaka Player handles adaptive-bitrate HLS playback. Shared TypeScript types are in `packages/types/`.

### Storage Provider Pattern

The `packages/storage/` package defines a provider interface implemented by local filesystem, S3/MinIO, and Wasabi backends. The API and Worker receive a concrete provider via dependency injection; adding a new storage backend means implementing the interface in that package.

### Auth Model

- **Access tokens:** Short-lived JWTs (golang-jwt/v5)
- **Refresh tokens:** Opaque tokens stored in Postgres with reuse detection (rotation invalidates the entire family on reuse)
- **Passwords:** Argon2id (never bcrypt)
- **Roles:** `admin`, `moderator`, `user` — enforced in [services/api/internal/http/middleware/](services/api/internal/http/middleware/)

### Transcoding Pipeline

1. Client uploads chunks → API assembles file → dispatches Redis job
2. Worker dequeues job → runs FFmpeg → produces HLS renditions (360p/480p/720p/1080p) + thumbnails + subtitles
3. Worker uploads HLS artifacts to configured storage provider
4. Worker reports progress via Redis; API polls and exposes to clients

### Live Streaming

OBS → RTMP → MediaMTX → HLS segments stored in DVR window. The API manages stream keys (stored in Postgres), queries MediaMTX for stream state, and serves a sliding-window DVR rewind. Live chat uses WebSocket connections with Redis pub/sub fanout.

## Configuration

All configuration is via environment variables. See [.env.example](.env.example) for the full list. Key groups:

- `DATABASE_URL` — Postgres connection string
- `REDIS_*` — Redis connection
- `STORAGE_PROVIDER` — `local`, `s3`, or `wasabi`
- `JWT_SECRET` — signing key for access tokens
- `MEDIAMTX_*` — live ingest service connection
- `SMTP_*` / `EMAIL_PROVIDER` — email delivery (`smtp` or `log`)

## Infrastructure

- **Dev:** Docker Compose ([docker-compose.yml](docker-compose.yml)) runs Postgres, Redis, MinIO, MediaMTX, Nginx
- **Prod:** [docker-compose.prod.yml](docker-compose.prod.yml) adds separate worker and live-helper containers
- **Installers:** [infra/installers/](infra/installers/) has Linux systemd scripts and a Windows PowerShell/WinSW installer
- **Nginx:** [infra/nginx/](infra/nginx/) holds the reverse-proxy config used in both dev and prod
