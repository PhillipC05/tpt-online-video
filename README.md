# TPT Online Video

**TPT Online Video** is an Apache 2.0-licensed, self-hostable online video platform inspired by YouTube — built as a real distributed media infrastructure project, not a simple CRUD upload site.

## Features

| Feature | Status |
|---|---|
| Resumable VOD uploads (presigned + chunked) | ✅ |
| FFmpeg transcoding pipeline with HLS renditions | ✅ |
| Adaptive bitrate playback (Shaka Player) | ✅ |
| PostgreSQL FTS search with filters and ranking | ✅ |
| Comments and engagement | ✅ |
| JWT + opaque refresh token auth with token rotation | ✅ |
| Argon2id password hashing | ✅ |
| Roles and permissions (admin / moderator / user) | ✅ |
| Full moderation workflow and audit log | ✅ |
| RTMP live ingest (OBS / ffmpeg) | ✅ |
| HLS live playback | ✅ |
| WebRTC/WHEP low-latency playback | ✅ (experimental) |
| Live DVR sliding-window rewind | ✅ |
| Real-time live chat (WebSocket) | ✅ |
| Abstracted storage (local filesystem / S3 / Wasabi) | ✅ |
| Docker Compose local dev stack | ✅ |
| Fully containerized production stack | ✅ |
| Windows self-contained installer | ✅ |
| Linux self-contained installer | ✅ |

## Architecture

```text
React Web UI (Vite + TypeScript)
    │
    ▼
Go API (chi, JWT, PostgreSQL, Redis)
    ├── PostgreSQL (metadata, auth, search, comments, moderation)
    ├── Redis (job queue, rate limiting, chat pub/sub)
    ├── Storage abstraction
    │       ├── Local filesystem
    │       ├── MinIO / S3-compatible
    │       └── Wasabi
    └── Transcoding queue
            │
            ▼
        Go Worker → FFmpeg → HLS renditions → Storage

OBS / Encoder
    │  (RTMP)
    ▼
MediaMTX
    ├── HLS live output
    ├── WebRTC/WHEP output
    └── DVR sliding window
```

## Repository Layout

```text
apps/
  web/                         React + TypeScript frontend

packages/
  auth/                        Argon2id hashing, JWT, token management
  media/                       FFmpeg transcoding helpers
  moderation/                  File type validation, ClamAV scanner interface
  search/                      Search provider abstraction (PostgreSQL FTS + Meilisearch)
  storage/                     Storage provider abstraction (local, S3, Wasabi)
  shared/                      Health check helpers

services/
  api/                         Go HTTP API
  worker/                      Go transcoding worker
  live/                        Go live-service helper

infra/
  installer/linux/             Systemd-based Linux installer
  installer/windows/           PowerShell/WinSW Windows installer
  nginx/                       Production reverse proxy configs

migrations/                    PostgreSQL migrations (golang-migrate)

docs/                          Full documentation
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.22+
- Node.js 20+ with pnpm
- FFmpeg (for local transcoding)

### Production (single command)

```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

Access the app at `http://localhost`.

### Development (hybrid)

```bash
# Start infrastructure
docker compose up -d postgres redis minio mediamtx

# Copy and edit environment
cp .env.example .env

# Run the API
cd services/api && go run ./cmd/tpt-api

# Run the worker (separate terminal)
cd services/worker && go run ./cmd/tpt-worker

# Run the frontend (separate terminal)
cd apps/web && pnpm install && pnpm dev
```

## Configuration

All configuration is via environment variables. See [`.env.example`](./.env.example) for the full list.

Key variables:

| Variable | Description | Default |
|---|---|---|
| `JWT_SECRET` | JWT signing secret (**must change in production**) | `change-me-in-development` |
| `LIVE_HOOK_SECRET` | MediaMTX webhook secret (**must change in production**) | `changeme-live-hook-secret` |
| `STORAGE_PROVIDER` | `local`, `s3`, or `wasabi` | `local` |
| `POSTGRES_SSLMODE` | PostgreSQL SSL mode | `disable` |
| `EMAIL_PROVIDER` | `log`, `smtp` | `log` |
| `APP_ENV` | `development` or `production` | `development` |

## Deployment Guides

| Platform | Guide |
|---|---|
| Any VPS (generic) | [docs/deployment/generic-vps.md](./docs/deployment/generic-vps.md) |
| Linode | [docs/deployment/linode.md](./docs/deployment/linode.md) |
| Windows Desktop | [docs/deployment/windows-desktop.md](./docs/deployment/windows-desktop.md) |
| Linux installer | [docs/installer.md](./docs/installer.md) |

## Documentation

- [Architecture](./docs/architecture.md)
- [API Reference](./docs/api.md)
- [Developer Guide](./docs/developer-guide.md)
- [Live Streaming](./docs/live-streaming.md)
- [Live Chat](./docs/live-chat.md)
- [Live DVR](./docs/live-dvr.md)
- [Storage Providers](./docs/storage-providers.md)
- [Moderation](./docs/moderation.md)
- [Security](./docs/security.md)
- [Testing](./docs/testing.md)
- [Roadmap](./docs/roadmap.md)

## License

[Apache 2.0](./LICENSE)
