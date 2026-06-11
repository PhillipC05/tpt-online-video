# Module Boundaries

This document defines the responsibility boundaries between all services and packages in the TPT Online Video monorepo.

## Services

### API (`services/api/`)
**Owns:** HTTP routing, auth/sessions, user/profile metadata, video metadata, upload session orchestration, transcoding job creation, search API, comments API, moderation API, live stream metadata API, WebSocket chat coordination.

**Must NOT:** Execute FFmpeg, manage MediaMTX directly, store files (delegates to storage package).

### Worker (`services/worker/`)
**Owns:** Queue consumption (Redis), FFmpeg process execution, HLS rendition generation, thumbnail generation, media metadata extraction, transcoding progress reporting, retry/dead-letter handling.

**Must NOT:** Serve HTTP, manage live streams, write to PostgreSQL directly (reports progress via API or Redis).

### Live Helper (`services/live/`)
**Owns:** MediaMTX lifecycle coordination, stream key generation/hashing, live stream start/end detection hooks, DVR sliding-window coordination, live chat integration hooks.

**Must NOT:** Serve user-facing HTTP, execute FFmpeg, manage VOD transcoding.

## Packages

### `packages/shared/`
**Owns:** Cross-cutting Go types, health check structures, common constants, utility functions used by multiple services.

### `packages/storage/`
**Owns:** Storage abstraction layer (local filesystem, S3-compatible, Wasabi). Provides a common interface for reading/writing media files and manifests.

### `packages/search/`
**Owns:** Search abstraction layer. Currently provides PostgreSQL full-text search provider; designed for Meilisearch provider later.

### `packages/auth/`
**Owns:** Password hashing (Argon2id), session token generation/validation, JWT handling if introduced later.

### `packages/media/`
**Owns:** Media metadata types, transcoding queue enqueue/dequeue operations, job status tracking. Bridges API with Worker.

### `packages/moderation/`
**Owns:** Moderation types, flagging logic, abuse detection helpers, automated moderation rules.

## Frontend

### `apps/web/`
**Owns:** React SPA with Shaka Player for HLS playback. Upload UI, watch page, home/landing page.

**Must NOT:** Access databases or storage directly. All data flows through the API.

## Shared TypeScript Types

### `packages/types/`
**Owns:** TypeScript type definitions shared between frontend and any TypeScript tooling. Mirrors relevant Go types from `packages/shared/`.

### `packages/tsconfig/`
**Owns:** Shared TypeScript compiler configuration presets (base.json, react.json) used across TS packages.

## Dependency Flow

```
apps/web ──► services/api ──► packages/shared
                │                    │
                ├──► packages/auth   │
                ├──► packages/search │
                ├──► packages/storage│
                └──► packages/media  │
                                     │
services/worker ──► packages/shared  │
                └──► packages/media  │
                                     │
services/live ───► packages/shared   │
```

Packages depend only on `packages/shared`. No package depends on a service. No service depends on another service.