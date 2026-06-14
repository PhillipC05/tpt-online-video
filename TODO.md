# TPT Online Video — Master TODO Checklist

Project: **TPT Online Video**  
License target: **MIT**  
Goal: Build a public, self-hostable, open-source YouTube-like platform with VOD, adaptive streaming, live streaming, moderation, abstracted storage/search, and Windows/Linux self-contained installers.

## Status Legend

- `[x]` Completed
- `[ ]` Remaining
- `[-]` In progress / partially complete
- `[~]` Deferred to later milestone

---

# Completed

## Project Definition

- [x] Define project name: **TPT Online Video**
- [x] Confirm project type: open-source online video platform
- [x] Confirm license target: **MIT**
- [x] Confirm primary goal:
  - [x] Portfolio-defining systems project
  - [x] Usable self-hosted YouTube-like platform
  - [x] Reference architecture for distributed media infrastructure
- [x] Confirm public v1 scope includes:
  - [x] VOD upload
  - [x] Transcoding pipeline
  - [x] HLS adaptive bitrate playback
  - [x] Users/auth
  - [x] Search
  - [x] Comments
  - [x] Full moderation workflow
  - [x] RTMP live ingest
  - [x] HLS live playback
  - [x] WebRTC low-latency playback
  - [x] Live DVR
  - [x] Live chat
  - [x] Admin dashboard
  - [x] Docker Compose deployment
  - [x] Windows self-contained installer
  - [x] Linux self-contained installer
- [x] Confirm monorepo approach
- [x] Confirm Go backend preference
- [x] Confirm frontend recommendation:
  - [x] React
  - [x] TypeScript
  - [x] Vite
  - [x] Shaka Player for HLS/DASH
  - [x] Native WebRTC path for low-latency live playback
- [x] Confirm storage abstraction targets:
  - [x] Local filesystem
  - [x] S3-compatible storage
  - [x] Wasabi
- [x] Confirm search approach:
  - [x] Abstracted search interface
  - [x] PostgreSQL full-text search first
  - [x] Meilisearch later
- [x] Confirm target installer OSes:
  - [x] Windows
  - [x] Linux
- [x] Confirm deployment targets:
  - [x] Hybrid deployment
  - [x] Docker Compose local/developer deployment
  - [x] Fully containerized production deployment (docker-compose.prod.yml)
  - [x] Self-contained installer
  - [x] Architecture designed for scale, demo can remain local/small
- [x] Confirm live streaming requirements:
  - [x] RTMP ingest from OBS
  - [x] HLS live playback
  - [x] WebRTC/sub-second playback
  - [x] DVR rewind/pause
  - [x] Live chat
- [x] Confirm auth requirements:
  - [x] Email/password
  - [x] OAuth via Google/GitHub
- [x] Confirm moderation requirements:
  - [x] Full moderation workflow
  - [x] Reports
  - [x] Takedowns
  - [x] Banned users
  - [x] Audit log
  - [x] Admin dashboard
  - [x] Role-based access
- [x] Confirm CI is not required initially
- [x] Confirm workspace is greenfield
- [x] Define high-level architecture
- [x] Define release roadmap
- [x] Create this master TODO checklist

---

## Phase 0 — Repository Foundation

- [x] Create repository root structure
- [x] Add `README.md`
  - [x] Project overview
  - [x] Feature list
  - [x] Architecture summary
  - [x] Quick start links
  - [x] Development links
  - [x] Deployment links
  - [x] Deployment guides (DigitalOcean, Linode, generic VPS, Windows)
  - [x] Live streaming links
  - [x] Moderation links
  - [x] Contributing links
- [x] Add `LICENSE` with MIT license text
- [x] Add `.gitignore`
- [x] Add `.editorconfig`
- [x] Add root `Makefile` or task runner scripts
- [x] Add initial architecture documentation
  - [x] `docs/architecture.md`
  - [x] System context diagram
  - [x] Component diagram
  - [x] Data flow diagram
  - [x] VOD pipeline diagram
  - [x] Live pipeline diagram
- [x] Add developer documentation
  - [x] Prerequisites
  - [x] Docker Compose setup
  - [x] Local binary setup
  - [x] Environment variables
  - [x] Running API
  - [x] Running worker
  - [x] Running frontend
  - [x] Running live services
- [x] Add contributor documentation
  - [x] Contribution workflow
  - [x] Code style
  - [x] Commit style
  - [x] Testing expectations
  - [x] Documentation expectations
- [x] Add release documentation
  - [x] Versioning strategy
  - [x] Release checklist
  - [x] Installer checklist
  - [x] Migration checklist

## Phase 1 — Monorepo Structure

- [x] Create frontend app directory
  - [x] `apps/web`
  - [x] React + TypeScript + Vite
  - [x] TypeScript config
  - [x] ESLint config
  - [x] Prettier config
  - [x] CSS strategy (plain CSS)
- [x] Create backend service directory
  - [x] `services/api`
  - [x] `services/worker`
  - [x] `services/live`
- [x] Create shared package directory
  - [x] `packages/shared`
  - [x] `packages/storage`
  - [x] `packages/search`
  - [x] `packages/auth`
  - [x] `packages/media`
  - [x] `packages/moderation`
- [x] Create infrastructure directory
  - [x] `infra/docker`
  - [x] `infra/installer`
  - [x] `infra/nginx`
  - [x] `infra/mediamtx`
- [x] Create documentation directory
  - [x] `docs/`
  - [x] `docs/diagrams/`
  - [x] `docs/deployment/` (platform-specific deployment guides)
- [x] Define module boundaries
  - [x] API owns HTTP, auth, metadata, comments, search, moderation
  - [x] Worker owns media jobs and FFmpeg execution
  - [x] Live service owns MediaMTX lifecycle, live metadata, DVR integration
  - [x] Shared packages own cross-cutting abstractions
- [x] Decide dependency management strategy
  - [x] Go workspace setup
  - [x] pnpm workspace setup for frontend
  - [x] Shared TypeScript types (`packages/types`, `@tpt/types`)
  - [x] Shared TS config presets (`packages/tsconfig`, `@tpt/tsconfig`)

## Phase 2 — Docker Compose Foundation

- [x] Create `docker-compose.yml`
- [x] Add PostgreSQL service
  - [x] Healthcheck
  - [x] Persistent volume
  - [x] Default database/user
- [x] Add Redis service
  - [x] Healthcheck
  - [x] Persistent volume
  - [x] Redis Streams/pubsub usage
- [x] Add MinIO service
  - [x] Buckets:
    - [x] `tpt-media`
    - [x] `tpt-live`
    - [x] `tpt-cache`
  - [x] Healthcheck
  - [x] Persistent volume
- [x] Add MediaMTX service
  - [x] RTMP port
  - [x] WebRTC port
  - [x] HLS port
  - [x] Config file
- [x] Add Nginx cache service for Linux
  - [x] Reverse proxy config
  - [x] HLS cache rules
  - [x] Live cache rules
  - [x] Static asset cache rules
- [x] Add local binary run instructions
  - [x] Windows
  - [x] Linux

## Phase 3 — Backend API Skeleton

- [x] Create Go module for API
- [x] Add configuration loading
  - [x] Environment variables
  - [x] Defaults
  - [x] Validation
- [x] Add structured logging
- [x] Add graceful shutdown
- [x] Add health endpoint
  - [x] `/healthz`
  - [x] Postgres health
  - [x] Redis health
  - [x] Storage health
- [x] Add readiness endpoint
  - [x] `/readyz`
- [x] Add database connection setup
  - [x] PostgreSQL
  - [x] Connection pool
  - [x] Migrations runner
- [x] Add Redis connection setup
  - [x] Streams
  - [x] Pub/Sub
  - [x] Cache
- [x] Add request ID middleware
- [x] Add CORS middleware

## Phase 4 — Database Schema and Migrations

- [x] Create initial migration set
  - [x] `migrations/000001_init.up.sql`
  - [x] `migrations/000001_init.down.sql`
- [x] Add users tables
  - [x] `users`
  - [x] `oauth_accounts`
  - [x] `sessions`
  - [x] `refresh_tokens`
- [x] Add roles/permissions tables
  - [x] `roles`
  - [x] `permissions`
  - [x] `role_permissions`
  - [x] `user_roles`
- [x] Add video tables
  - [x] `videos`
  - [x] `video_renditions`
  - [x] `upload_sessions`
  - [x] `transcode_jobs`
- [x] Add comment tables
  - [x] `comments`
  - [x] `comment_reports`
- [x] Add live stream tables
  - [x] `live_streams`
  - [x] `live_chat_messages`
  - [x] `live_stream_reports`
- [x] Add moderation tables
  - [x] `moderation_reports`
  - [x] `moderation_actions`
  - [x] `audit_log`
- [x] Add search tables/indexes
  - [x] `search_documents`
  - [x] Postgres full-text indexes
- [x] Add indexes for common queries
  - [x] Videos by owner
  - [x] Videos by status
  - [x] Videos by visibility
  - [x] Videos by created date
  - [x] Comments by video
  - [x] Reports by status
  - [x] Live streams by owner/status

## Phase 5 — Auth System

- [x] Create `packages/auth/` module with Argon2id password hashing
- [x] Add auth Go module and interface definitions

## Phase 6 — Storage Abstraction

- [x] Define `StorageProvider` interface
- [x] Implement local filesystem provider
- [x] Implement S3-compatible provider
  - [x] MinIO
  - [x] AWS S3
  - [x] Wasabi
- [x] Implement storage layout helpers
- [x] Implement storage health checks
- [x] Add storage provider documentation

## Phase 7 — Upload System (Phase 1)

- [x] Define upload session model
- [x] Implement upload session creation
- [x] Implement resumable chunk upload
- [x] Implement upload completion
- [x] Implement upload progress tracking
- [x] Store raw upload in abstracted storage
- [x] Create transcoding job after upload completion

## Phase 8 — Transcoding Queue (Phase 1)

- [x] Choose queue implementation
  - [x] Redis Streams first
- [x] Define queue abstraction
- [x] Implement job creation
- [x] Implement job claiming
- [x] Implement job acknowledgment
- [x] Implement job retry

## Phase 9 — FFmpeg Worker (Phase 1)

- [x] Create Go module for worker
- [x] Add worker configuration
- [x] Add FFmpeg command builder
- [x] Add FFmpeg process runner
- [x] Parse FFmpeg stderr progress
- [x] Generate HLS renditions
  - [x] 1080p
  - [x] 720p
  - [x] 480p
  - [x] 360p
- [x] Extract duration and metadata
- [x] Write HLS manifests
- [x] Upload outputs to storage
- [x] Update database job status
- [x] Update video rendition status
- [x] Mark video as ready when complete
- [x] Handle transcoding failures
- [x] Add worker logs

## Phase 10 — VOD Metadata and Watch Experience (Phase 1)

- [x] Implement video metadata API
  - [x] Create video record
  - [x] Get video by ID
  - [x] List videos
- [x] Implement visibility states
  - [x] Public
  - [x] Unlisted
  - [x] Private
  - [x] Removed
- [x] Implement video status states
  - [x] Uploading
  - [x] Queued
  - [x] Transcoding
  - [x] Ready
- [x] Implement HLS manifest resolution
- [x] Implement thumbnail URLs
- [x] Implement watch page API payload
- [x] Add view count increment
  - [x] Basic counter first
- [x] Add frontend watch page
- [x] Add Shaka Player integration
- [x] Add adaptive quality selector
- [x] Add playback error handling

## Phase 12 — Search

- [x] Define `SearchProvider` interface
- [x] Implement PostgreSQL full-text search provider

## Phase 21 — CDN and Cache Layer

- [x] Add Nginx cache config for Linux (hybrid deployment)
  - [x] Reverse proxy config
  - [x] HLS cache rules
  - [x] Static asset cache rules
- [x] Add containerized nginx config (production deployment)
  - [x] API proxy
  - [x] WebSocket support
  - [x] HLS proxy to MediaMTX
  - [x] Static asset caching
- [x] Add Windows-compatible deployment guide
- [x] Add SSL/TLS configuration

## Phase 22 — Frontend App (Skeleton)

- [x] Create React routing
  - [x] Home
  - [x] Watch
  - [x] Upload
- [x] Create upload client
- [x] Create navigation/header
- [x] Create responsive layout
- [x] Create loading states
- [x] Create error states

## Phase 27 — Documentation

- [x] Write `README.md`
- [x] Write `docs/architecture.md`
- [x] Write `docs/developer-guide.md`
- [x] Write `docs/deployment.md`
- [x] Write `docs/storage-providers.md`
- [x] Write `docs/live-streaming.md`
- [x] Write `docs/moderation.md`
- [x] Write `docs/contributor-guide.md`
- [x] Write `docs/roadmap.md`
- [x] Write `docs/dependency-management.md`
- [x] Write `docs/module-boundaries.md`
- [x] Write `docs/target-audience.md`
- [x] Write deployment guides:
  - [x] `docs/deployment/digitalocean.md`
  - [x] `docs/deployment/linode.md`
  - [x] `docs/deployment/generic-vps.md`
  - [x] `docs/deployment/windows-desktop.md`
- [x] Add architecture diagrams
- [x] Add sequence diagrams
  - [x] Upload sequence
  - [x] Transcoding sequence
  - [x] Watch sequence
  - [x] Live broadcast sequence
  - [x] Live chat sequence
  - [x] Moderation action sequence

## Phase 28 — Windows Self-Contained Installer (Scaffolding)

- [x] Create installer directory structure `infra/installer/windows/`
- [x] Create PowerShell installer script (`install.ps1`)
- [x] Create winsw service configuration template
- [x] Write Windows deployment guide

## Phase 29 — Linux Self-Contained Installer (Scaffolding)

- [x] Create installer directory structure `infra/installer/linux/`
- [x] Create Bash installer script (`install.sh`)
- [x] Create systemd service units:
  - [x] `tpt-api.service`
  - [x] `tpt-worker.service`
  - [x] `tpt-live.service`
- [x] Create shared configuration template (`common/config.yaml`)

## Phase 30 — Production Docker Deployment

- [x] Create Dockerfiles for all Go services
  - [x] `services/api/Dockerfile` (multi-stage, alpine runtime)
  - [x] `services/worker/Dockerfile` (multi-stage, alpine runtime with FFmpeg)
  - [x] `services/live/Dockerfile` (multi-stage, alpine runtime)
- [x] Create frontend Dockerfile
  - [x] `apps/web/Dockerfile` (Node build + nginx:alpine runtime)
  - [x] `apps/web/nginx.conf` (frontend nginx config)
- [x] Create `docker-compose.prod.yml` (full production stack)
  - [x] Infrastructure services (postgres, redis, minio, mediamtx)
  - [x] TPT services (api, worker, live) with environment config
  - [x] Reverse proxy (nginx)
  - [x] Healthchecks on all services
  - [x] Named volumes for persistence
- [x] Create `.dockerignore` files for all services
- [x] Create containerized nginx config (`infra/docker/nginx/tpt-prod.conf`)
- [x] Update Makefile with Docker build/production targets
- [x] Update README with deployment guides table and single-command quick start

---

# Remaining

## Phase 2 — Docker Compose Foundation (continued)

- [ ] Add API service in docker-compose.yml (development)
  - [ ] Environment config
  - [ ] Depends on Postgres/Redis/MinIO
- [ ] Add worker service in docker-compose.yml (development)
  - [ ] Environment config
  - [ ] Depends on API/Redis/MinIO
- [ ] Add frontend dev service in docker-compose.yml (development)
  - [ ] Vite dev server
  - [ ] Proxy to API

## Phase 3 — Backend API Skeleton (continued)

- [ ] Add OpenAPI specification
- [ ] Add API response envelope
- [ ] Add error handling middleware
- [ ] Add rate limiting middleware
- [ ] Add authentication middleware
- [ ] Add authorization middleware
- [ ] Add admin middleware
- [ ] Add WebSocket middleware

## Phase 5 — Auth System (continued)

- [ ] Implement email/password registration
- [ ] Implement email/password login
- [ ] Implement JWT access tokens
- [ ] Implement refresh tokens stored in database
- [ ] Implement token rotation
- [ ] Implement logout
- [ ] Implement password reset flow
  - [ ] Token generation
  - [ ] Email provider abstraction
  - [ ] Local SMTP/dev mode
- [ ] Implement OAuth providers
  - [ ] Google
  - [ ] GitHub
- [ ] Implement OAuth account linking
- [ ] Implement session revocation
- [ ] Implement role-based access control
- [ ] Implement admin seed account flow
- [ ] Add auth tests

## Phase 6 — Storage Abstraction (continued)

- [x] Implement multipart upload support
- [x] Implement presigned upload URLs
- [x] Implement presigned download URLs
- [x] Implement object metadata handling
- [x] Implement delete operations
- [x] Implement list operations
- [x] Add storage provider tests

## Phase 7 — Upload System (continued)

- [x] Implement upload cancellation
- [x] Implement upload expiration
- [x] Implement virus/malware scanning hook interface
  - [x] No-op default
  - [x] ClamAV adapter later
- [x] Implement file type validation
- [x] Implement file size validation
- [x] Add upload API tests

## Phase 8 — Transcoding Queue (continued)

- [x] Implement dead-letter handling
- [x] Implement worker heartbeat
- [x] Implement queue metrics
- [x] Implement dynamic worker scaling controller
  - [x] Queue depth metric
  - [x] CPU usage metric
  - [x] Min worker count
  - [x] Max worker count
  - [x] Scale-up policy
  - [x] Scale-down policy
- [x] Add queue tests

## Phase 9 — FFmpeg Worker (continued)

- [x] Calculate transcoding percentage
- [x] Generate thumbnails/posters
- [x] Implement retryable failure classification
- [x] Implement non-retryable failure classification
- [x] Add worker metrics
- [x] Add worker tests using sample media files

## Phase 10 — VOD Metadata and Watch Experience (continued)

- [x] Implement update video metadata
- [x] Implement delete/unpublish video
- [x] Implement signed media URLs
- [x] Add related videos query
- [x] Add async analytics for view counts
- [x] Add video metadata editing UI

## Phase 11 — ABR Player Enhancements

- [x] Add player bandwidth metrics
- [x] Add player buffering metrics
- [x] Add quality switch events
- [x] Add error events
- [x] Add keyboard shortcuts
- [x] Add captions/subtitles support later
- [x] Add DASH support later
- [x] Add custom player skin
- [x] Add demo overlay showing selected rendition and bitrate

## Phase 12 — Search

- [x] Define `SearchProvider` interface
- [x] Implement PostgreSQL full-text search provider
- [x] Implement search query model
- [x] Implement search result model
- [x] Implement ranking
  - [x] Text relevance
  - [x] Recency
  - [x] View count
  - [x] Engagement
- [x] Implement search indexing on video publish/update/delete
- [x] Implement search API
- [x] Add filters
  - [x] Duration
  - [x] Upload date
  - [x] Live/VOD
  - [x] Owner/channel
- [x] Add frontend search page
- [x] Add JSON tags for correct serialization
- [x] Fix frontend API envelope unwrapping
- [x] Add search autocomplete
  - [x] Backend - Autocomplete method on Provider interface
  - [x] Backend - PostgreSQL ILIKE autocomplete implementation
  - [x] Backend - Autocomplete HTTP handler
  - [x] Backend - Route at GET /api/v1/search/autocomplete
  - [x] Frontend - SearchAutocomplete component with debounce, keyboard nav, click-outside
  - [x] Frontend - CSS styles for autocomplete dropdown
- [x] Add Meilisearch adapter
  - [x] Search, Autocomplete, IndexVideo, DeleteVideo, Health
  - [x] Filter support (duration, upload date, media type, owner)
  - [x] Sort support (relevance, recent, views)
  - [x] Document mapping (meiliDocument <-> Video)
  - [x] Helper field extractors for Meilisearch response

## Phase 13 — Comments and Engagement

- [ ] Implement comment creation
- [ ] Implement comment listing
- [ ] Implement comment editing
- [ ] Implement comment deletion
- [ ] Implement comment reports
- [ ] Implement likes/reactions
- [ ] Implement view counts
- [ ] Implement subscriptions/channels later
- [ ] Add frontend comment section
- [ ] Add moderation hooks for comments

## Phase 14 — Channels and Profiles

- [ ] Implement channel/profile model
- [ ] Implement profile update
- [ ] Implement avatar upload
- [ ] Implement banner upload
- [ ] Implement channel page
- [ ] Implement channel videos listing
- [ ] Implement channel live streams listing
- [ ] Implement subscription model later

## Phase 15 — Moderation System

- [ ] Implement roles and permissions
- [ ] Implement admin dashboard API
- [ ] Implement moderation report creation
  - [ ] Video reports
  - [ ] Comment reports
  - [ ] User reports
  - [ ] Live chat reports
  - [ ] Live stream reports
- [ ] Implement report queue
- [ ] Implement report assignment
- [ ] Implement report resolution
- [ ] Implement moderation actions
  - [ ] Hide content
  - [ ] Unpublish video
  - [ ] Delete video
  - [ ] Remove comment
  - [ ] Terminate live stream
  - [ ] Lock live chat
  - [ ] Suspend user
  - [ ] Ban user
  - [ ] Restore content
- [ ] Implement audit log
- [ ] Implement admin notes
- [ ] Implement appeal status field
- [ ] Implement moderation dashboard UI
- [ ] Implement report filtering
- [ ] Implement moderation action history
- [ ] Add moderation tests
- [ ] Add moderation policy template documentation

## Phase 16 — Live Streaming Foundation

- [x] Choose live media server
  - [x] MediaMTX
- [x] Add MediaMTX configuration
- [x] Add RTMP ingest documentation
- [x] Generate stream keys
- [x] Hash stream keys before storing
- [x] Implement live stream creation
- [x] Implement live stream update
- [x] Implement live stream deletion
- [x] Implement live stream start detection
- [x] Implement live stream end detection
- [x] Implement live HLS URL generation
- [x] Implement live WebRTC URL generation
- [x] Implement live stream metadata API
- [ ] Add frontend live creation page
- [x] Add OBS setup guide

## Phase 17 — Live HLS Playback

- [x] Generate live HLS manifest URLs
- [x] Add Shaka Player live HLS support
- [x] Add live badge/status
- [x] Add reconnect handling
- [x] Add latency indicator
- [ ] Add viewer count later

## Phase 18 — WebRTC Low-Latency Playback

- [x] Configure MediaMTX WebRTC/WHEP
- [x] Implement WebRTC playback API endpoint if needed
- [x] Add frontend WebRTC player path
- [x] Add fallback to HLS if WebRTC unavailable
- [x] Add WebRTC latency metrics
- [x] Add WebRTC troubleshooting docs
- [x] Mark WebRTC as experimental until hardened

## Phase 19 — Live DVR

- [x] Define DVR window configuration
- [x] Implement sliding HLS playlist
- [x] Store recent live segments
  - [x] Local disk first
  - [ ] Redis later
  - [ ] Object storage later
- [x] Implement segment retention cleanup
- [x] Implement live pause/rewind behavior
- [x] Implement jump-to-live behavior
- [x] Add frontend DVR controls
- [ ] Add DVR performance tests
- [x] Add DVR documentation

## Phase 20 — Live Chat

- [x] Implement WebSocket chat rooms
- [x] Implement chat message creation
- [x] Implement Redis pub/sub broadcast
- [x] Persist chat messages
- [x] Implement message history loading
- [x] Implement chat moderation
  - [x] Delete message
  - [x] Timeout user
  - [x] Ban user from chat
  - [x] Lock chat
- [x] Implement chat reports
- [x] Add frontend live chat UI
- [ ] Add chat typing indicator later
- [x] Add chat moderation UI
- [x] Add chat sync documentation

## Phase 22 — Frontend App (continued)

- [x] Create API client
- [x] Create auth client
- [x] Create WebSocket client
- [x] Create video card components
- [x] Create video grid components
- [x] Create search page
- [x] Create channel page
- [x] Create live page
- [x] Create admin pages
- [x] Create moderation pages
- [x] Create live streaming pages
- [x] Create dark/light theme
- [x] Create empty states
- [x] Create forms
- [x] Add accessibility checks
- [x] Add frontend tests

## Phase 23 — Admin Dashboard

- [x] Implement admin home
- [x] Implement user management
- [x] Implement video management
- [x] Implement comment management
- [x] Implement report queue
- [x] Implement moderation action history
- [x] Implement audit log viewer
- [x] Implement system status page
  - [x] API health
  - [x] Postgres health
  - [x] Redis health
  - [x] Storage health
  - [x] Queue depth
  - [x] Worker count
  - [x] Live stream count
- [x] Implement admin settings
  - [x] Storage provider settings
  - [x] Search provider settings
  - [x] Moderation settings
  - [x] Live settings

## Phase 24 — Metrics and Observability

- [x] Add API metrics
  - [x] Request count
  - [x] Request duration
  - [x] Error count
  - [x] Auth failures
- [x] Add worker metrics
  - [x] Jobs processed
  - [x] Jobs failed
  - [x] Queue depth (via admin system/status)
  - [x] Transcoding duration
  - [x] FFmpeg failures
- [x] Add live metrics
  - [x] Active streams
  - [x] Active viewers
  - [x] Chat messages
  - [x] DVR segment count (dvr_enabled_streams gauge)
- [ ] Add Prometheus metrics endpoint later
- [ ] Add Grafana dashboard later
- [x] Add structured log correlation
- [x] Add audit log query helpers

## Phase 25 — Security

- [x] Validate all user input
- [x] Sanitize rich text if used
- [x] Prevent open redirects
- [x] Secure OAuth state/PKCE
- [x] Secure stream keys
- [x] Secure presigned URLs
- [x] Add upload size limits
- [x] Add rate limits
- [x] Add brute-force protection
- [x] Add CORS policy
- [x] Add CSRF strategy if cookie sessions are used
- [x] Add content ownership checks
- [x] Add admin authorization checks
- [x] Add secret rotation docs
- [x] Add security checklist

## Phase 26 — Testing

- [ ] Add backend unit tests
- [ ] Add backend integration tests
- [ ] Add storage provider tests
- [ ] Add search provider tests
- [ ] Add auth tests
- [ ] Add upload tests
- [ ] Add transcoding worker tests
- [ ] Add moderation tests
- [ ] Add live service tests
- [ ] Add frontend unit tests
- [ ] Add frontend component tests
- [ ] Add end-to-end tests later
- [ ] Add sample media fixtures
- [ ] Add testcontainers setup later

## Phase 27 — Documentation (continued)

- [ ] Write `docs/installer.md`
- [ ] Write `docs/security.md`
- [ ] Write `docs/testing.md`
- [ ] Add API documentation
- [ ] Add admin user guide
- [ ] Add broadcaster guide
- [ ] Add moderator guide
- [ ] Add maintainer guide

## Phase 28 — Windows Self-Contained Installer (continued)

- [ ] Choose installer technology
  - [ ] Inno Setup
  - [ ] WiX
  - [ ] Tauri bundler if suitable
- [ ] Package frontend static assets
- [ ] Package Go API binary
- [ ] Package Go worker binary
- [ ] Package live helper binary
- [ ] Package PostgreSQL
- [ ] Package Redis
- [ ] Package MinIO
- [ ] Package FFmpeg
- [ ] Package MediaMTX
- [ ] Package cache layer
- [ ] Configure Windows services
  - [ ] PostgreSQL
  - [ ] Redis
  - [ ] MinIO
  - [ ] TPT API
  - [ ] TPT Worker
  - [ ] MediaMTX
- [ ] Configure data directories
- [ ] Configure logs directory
- [ ] Configure environment files
- [ ] Configure firewall rules if needed
- [ ] Add installer health check
- [ ] Add uninstaller cleanup
- [ ] Add upgrade path
- [ ] Test on Windows 10/11
- [ ] Write Windows installer documentation

## Phase 29 — Linux Self-Contained Installer (continued)

- [ ] Choose installer technology
  - [ ] `.deb`
  - [ ] `.rpm`
  - [ ] AppImage
  - [ ] Shell installer
- [ ] Package frontend static assets
- [ ] Package Go API binary
- [ ] Package Go worker binary
- [ ] Package live helper binary
- [ ] Package PostgreSQL
- [ ] Package Redis
- [ ] Package MinIO
- [ ] Package FFmpeg
- [ ] Package MediaMTX
- [ ] Package Nginx cache layer
- [ ] Configure systemd services
  - [ ] PostgreSQL
  - [ ] Redis
  - [ ] MinIO
  - [ ] TPT API
  - [ ] TPT Worker
  - [ ] MediaMTX
  - [ ] Nginx
- [ ] Configure data directories
- [ ] Configure logs directory
- [ ] Configure environment files
- [ ] Configure firewall rules if needed
- [ ] Add installer health check
- [ ] Add uninstaller cleanup
- [ ] Add upgrade path
- [ ] Test on Ubuntu/Debian
- [ ] Test on Fedora/RHEL if feasible
- [ ] Write Linux installer documentation

## Phase 31 — Public v1 Hardening

- [ ] Verify VOD upload works end-to-end
- [ ] Verify transcoding progress is accurate
- [ ] Verify HLS playback works across browsers
- [ ] Verify adaptive bitrate switching works
- [ ] Verify search works
- [ ] Verify comments work
- [ ] Verify moderation workflow works
- [ ] Verify admin dashboard works
- [ ] Verify RTMP live ingest works
- [ ] Verify HLS live playback works
- [ ] Verify WebRTC playback works or is clearly marked experimental
- [ ] Verify live DVR works
- [ ] Verify live chat works
- [ ] Verify local filesystem storage works
- [ ] Verify S3-compatible storage works
- [ ] Verify Wasabi storage works
- [ ] Verify Docker Compose hybrid deployment works
- [ ] Verify Docker Compose production deployment works
- [ ] Verify Windows installer works
- [ ] Verify Linux installer works
- [ ] Verify documentation is complete
- [ ] Verify MIT license is present
- [ ] Prepare public release notes
- [ ] Prepare demo seed data
- [ ] Prepare screenshots/screenshare/demo script

---

# Public v1 Acceptance Criteria

- [ ] MIT license exists
- [ ] Repository has comprehensive documentation
- [ ] Docker Compose starts the core stack
- [ ] Docker Compose production stack starts all services
- [ ] API starts successfully
- [ ] Worker starts successfully
- [ ] Frontend starts successfully
- [ ] Users can register and log in
- [ ] Users can authenticate with OAuth
- [ ] Users can upload videos
- [ ] Uploads are stored through abstracted storage
- [ ] Videos are transcoded into multiple HLS renditions
- [ ] Transcoding progress is visible in real time
- [ ] Users can watch videos with adaptive bitrate playback
- [ ] Users can search videos
- [ ] Users can comment
- [ ] Admins can moderate reports
- [ ] Admins can ban/suspend users
- [ ] Admins can unpublish/remove videos
- [ ] Broadcasters can stream from OBS using RTMP
- [ ] Viewers can watch live streams through HLS
- [ ] Viewers can watch live streams through WebRTC
- [ ] Live chat works in real time
- [ ] Live DVR allows pause/rewind within configured window
- [ ] Storage works with local filesystem
- [ ] Storage works with S3-compatible provider
- [ ] Storage works with Wasabi
- [ ] Windows self-contained installer exists
- [ ] Linux self-contained installer exists
- [ ] Public v1 release notes exist

---

# Known Risks and Mitigations

## Installer complexity

- [ ] Risk acknowledged: bundling PostgreSQL, Redis, MinIO, FFmpeg, MediaMTX, frontend, backend, worker, and cache layer is complex.
- [ ] Mitigation: build Linux installer first, then Windows.
- [ ] Mitigation: keep installer logic separate from application logic.
- [ ] Mitigation: use a Go-based launcher/service manager.

## Windows cache layer

- [ ] Risk acknowledged: Nginx/Varnish are not ideal on Windows.
- [ ] Mitigation: use Nginx on Linux.
- [ ] Mitigation: use Caddy or Go-based cache layer on Windows.
- [ ] Mitigation: document platform differences.

## Live streaming complexity

- [ ] Risk acknowledged: RTMP + HLS is manageable, but WebRTC + DVR + live chat is significantly harder.
- [ ] Mitigation: implement RTMP + HLS first.
- [ ] Mitigation: implement WebRTC second.
- [ ] Mitigation: implement DVR third.
- [ ] Mitigation: polish chat sync last.
- [ ] Mitigation: mark WebRTC/DVR experimental until hardened.

## Moderation policy complexity

- [ ] Risk acknowledged: moderation workflow does not solve policy/legal decisions automatically.
- [ ] Mitigation: provide templates and audit logs.
- [ ] Mitigation: keep moderation actions configurable.
- [ ] Mitigation: document that deployers are responsible for their own policies.

## Scale expectations

- [ ] Risk acknowledged: public v1 will not match YouTube-scale infrastructure.
- [ ] Mitigation: design service boundaries for horizontal scaling.
- [ ] Mitigation: use stateless API and worker processes.
- [ ] Mitigation: use object storage for media.
- [ ] Mitigation: use cache/CDN patterns for viral traffic.
- [ ] Mitigation: document scaling roadmap.

---

# Suggested Build Order

1. Repository foundation
2. Docker Compose hybrid stack
3. Docker Compose production stack
4. Backend API skeleton
5. Database schema and migrations
6. Auth
7. Storage abstraction
8. Upload system
9. Transcoding queue
10. FFmpeg worker
11. VOD metadata and watch page
12. Search
13. Comments
14. Moderation
15. Live RTMP/HLS
16. Live chat
17. WebRTC
18. DVR
19. Admin dashboard polish
20. Linux installer
21. Windows installer
22. Public v1 hardening and release

---

# Current Next Tasks

- [x] Repository root structure
- [x] MIT license
- [x] README with deployment guides table and single-command quick start
- [x] Makefile / task runner
- [x] Architecture documentation and diagrams
- [x] Developer guide
- [x] Contributor guide
- [x] Release process documentation
- [x] Target audience documentation
- [x] Module boundaries documentation
- [x] Dependency management documentation
- [x] Docker Compose foundation (dev/hybrid)
- [x] Go API skeleton
- [x] React frontend skeleton with ESLint + Prettier
- [x] Initial database schema
- [x] Storage abstraction (local, S3, Wasabi)
- [x] Search abstraction (PostgreSQL FTS)
- [x] Auth foundation (Argon2id)
- [x] Upload pipeline (Phase 1)
- [x] Transcoding queue (Phase 1)
- [x] FFmpeg worker (Phase 1)
- [x] Watch page with Shaka Player (Phase 1)
- [x] Shared TypeScript types (`@tpt/types`)
- [x] Shared TS config presets (`@tpt/tsconfig`)
- [x] pnpm workspace setup
- [x] Dockerfiles for all services (multi-stage builds)
- [x] Production Docker Compose stack (`docker-compose.prod.yml`)
- [x] Containerized nginx reverse proxy config
- [x] Linux installer scaffolding (systemd units + install script)
- [x] Windows installer scaffolding (winsw + PowerShell)
- [x] Platform deployment guides (DigitalOcean, Linode, generic VPS, Windows Desktop)
- [x] Nginx configs for hybrid and containerized deployment
- [x] SSL/TLS configuration for production
- [ ] Email/password registration and login (Phase 2)
- [ ] JWT access + refresh token flow (Phase 2)
- [ ] Auth middleware for protected routes (Phase 2)
- [ ] Admin/moderator seed users (Phase 2)