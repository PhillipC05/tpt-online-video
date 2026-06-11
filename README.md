# TPT Online Video

**TPT Online Video** is an MIT-licensed, self-hostable online video platform inspired by YouTube, built as a systems-engineering project rather than a simple CRUD upload site.

The goal is to provide a public, self-hosted platform with real distributed media infrastructure:

- VOD upload and resumable uploads
- FFmpeg-based transcoding pipeline
- Adaptive bitrate HLS playback
- Abstracted object storage for local filesystem, S3-compatible providers, and Wasabi
- Redis-backed job queue with worker scaling design
- PostgreSQL metadata, auth, search, comments, and moderation
- RTMP live ingest from OBS
- HLS live playback
- WebRTC low-latency playback path
- Live DVR sliding-window design
- Real-time live chat
- Full moderation workflow and audit log
- Docker Compose local deployment
- Fully containerized production deployment (single `docker compose up`)
- Windows and Linux self-contained installer

## Status

This repository is currently in the foundational scaffolding phase.

Completed:

- Project vision and architecture plan
- MIT license
- Master TODO checklist
- Initial repository structure
- Docker Compose foundation
- Go service skeletons
- React frontend skeleton
- Initial database schema
- Storage/search/auth abstractions
- Production Docker Compose stack
- Platform deployment guides (DigitalOcean, Linode, generic VPS, Windows)

Remaining major work:

- Full upload/transcoding pipeline
- Full auth implementation
- Full moderation workflow
- Full live streaming integration
- Installer packaging
- Public v1 hardening
- Federation / ActivityPub (future)

See [`TODO.md`](./TODO.md) for the complete task checklist.

## High-Level Architecture

```text
React Web UI
    |
    v
Go API
    |
    |--> PostgreSQL
    |--> Redis
    |--> Storage Abstraction
    |       |--> Local filesystem
    |       |--> S3-compatible
    |       |--> Wasabi
    |
    |--> Search Abstraction
    |       |--> PostgreSQL FTS first
    |       |--> Meilisearch later
    |
    |--> Transcoding Queue
            |
            v
        Go Worker
            |
            v
          FFmpeg
            |
            v
        HLS Renditions

OBS / Live Encoder
    |
    v
MediaMTX
    |
    |--> HLS live output
    |--> WebRTC/WHEP output
    |--> DVR sliding window
```

## Repository Layout

```text
apps/
  web/                         React + TypeScript frontend (@tpt/web)

packages/
  shared/                      Shared Go types/utilities
  storage/                     Storage provider abstraction
  search/                      Search provider abstraction
  auth/                        Auth primitives and interfaces
  media/                       Media/transcoding helpers
  moderation/                  Moderation primitives
  types/                       Shared TypeScript types (@tpt/types)
  tsconfig/                    Shared TypeScript config presets (@tpt/tsconfig)

services/
  api/                         Go HTTP API
  worker/                      Go transcoding worker
  live/                        Go live-service helper

infra/
  docker/                      Docker Compose and service configs
    mediamtx/                  MediaMTX configuration
    nginx/                     Containerized nginx config
  installer/                   Self-contained installer scripts
    linux/                     Systemd-based Linux installer
    windows/                   PowerShell/winsw Windows installer
    common/                    Shared configuration templates
  nginx/                       Production reverse proxy configs (for hybrid deployment)

docs/
  architecture.md
  developer-guide.md
  deployment.md
  deployment/                  Platform-specific deployment guides
  storage-providers.md
  live-streaming.md
  moderation.md
  roadmap.md
  module-boundaries.md
  dependency-management.md
  diagrams/

migrations/
  000001_init.up.sql
  000001_init.down.sql
```

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.22+
- Node.js 20+
- pnpm or npm
- FFmpeg for local worker testing

### Fully containerized (single command)

```bash
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

Access the app at `http://localhost`.

### Hybrid (development)

```bash
# Start infrastructure
docker compose up postgres redis minio mediamtx

# Run the API
cd services/api && go run ./cmd/tpt-api

# Run the frontend (separate terminal)
cd apps/web && npm install && npm run dev
```

## Deployment Guides

| Platform | Guide |
|---|---|
| DigitalOcean | [docs/deployment/digitalocean.md](./docs/deployment/digitalocean.md) |
| Linode | [docs/deployment/linode.md](./docs/deployment/linode.md) |
| Any VPS | [docs/deployment/generic-vps.md](./docs/deployment/generic-vps.md) |
| Windows Desktop | [docs/deployment/windows-desktop.md](./docs/deployment/windows-desktop.md) |

## Dependency Management

**Go backend:** Go workspace (`go.work`) manages all Go modules under `packages/` and `services/`.

**Frontend:** pnpm workspace manages the React frontend and shared TypeScript packages.

See [`docs/dependency-management.md`](./docs/dependency-management.md) for details.

## Documentation

- [Architecture](./docs/architecture.md)
- [Module Boundaries](./docs/module-boundaries.md)
- [Dependency Management](./docs/dependency-management.md)
- [Deployment](./docs/deployment.md)
- [Developer Guide](./docs/developer-guide.md)
- [Storage Providers](./docs/storage-providers.md)
- [Live Streaming](./docs/live-streaming.md)
- [Moderation](./docs/moderation.md)
- [Roadmap](./docs/roadmap.md)
- [Master TODO](./TODO.md)

## License

MIT. See [`LICENSE`](./LICENSE).