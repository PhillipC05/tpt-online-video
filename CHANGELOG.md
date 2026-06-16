# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.0] — 2026-06-16

### Added

**Core platform**
- VOD upload pipeline: resumable chunked uploads and presigned S3 PUT for files under 100 MB
- FFmpeg transcoding worker with multiple HLS renditions (360p, 480p, 720p, 1080p)
- Real-time transcoding progress via Redis pub/sub
- Adaptive bitrate HLS playback with Shaka Player
- PostgreSQL full-text search with ranking and filter support
- Comments and engagement (likes/dislikes)
- Video thumbnails and subtitle extraction

**Authentication & security**
- JWT access tokens (15-minute TTL) stored in memory; tokens delivered via HttpOnly cookie
- Opaque refresh tokens with rotation, reuse detection, and family revocation
- Argon2id password hashing (64 MB memory, 3 iterations)
- Dedicated `ChangePassword` flow with current-password verification
- Password reset via time-limited, single-use tokens (email address no longer embedded in reset URL)
- Session revocation with user-ownership enforcement

**Live streaming**
- RTMP ingest via MediaMTX (compatible with OBS and ffmpeg)
- HLS live playback
- WebRTC/WHEP low-latency playback path (experimental)
- Live DVR sliding-window pause/rewind
- Real-time live chat over WebSocket with Redis pub/sub fan-out

**Moderation & admin**
- Report queue with resolution workflow
- User ban/suspend/restore actions
- Content unpublish and permanent deletion
- Moderator and admin audit log
- Admin dashboard: user management, video management, comment management, system status

**Storage**
- Storage provider abstraction: local filesystem, S3-compatible, Wasabi
- Proper `UploadHLSRenditions` using `filepath.Walk` + `PutObject` (replaces non-functional stub)

**Infrastructure**
- Docker Compose development stack (PostgreSQL, Redis, MinIO, MediaMTX, Nginx)
- Fully containerized production stack (`docker-compose.prod.yml`)
- Linux systemd installer with health checks, upgrade, and uninstall scripts
- Windows self-contained installer (WinSW + PowerShell)

**Configuration**
- `LIVE_HOOK_SECRET` is now validated in production (must be changed from default)
- `POSTGRES_SSLMODE` is configurable (default `disable`; set to `require` for cloud deployments)
- Email provider (`EMAIL_PROVIDER=smtp`) is now fully wired from config — no longer hardcoded to `log`

### Fixed

- **Critical**: `getUserID()` in upload handlers used a plain-string context key instead of the typed middleware key, causing all upload endpoints to return HTTP 401 for authenticated users
- **Critical**: `UploadChunk` did not verify session ownership — any authenticated user could upload into another user's session
- **Critical**: `ChangePassword` called `ResetPassword` with an empty token, always returning HTTP 500; replaced with a dedicated `Service.ChangePassword` method
- **High**: `RevokeSession` ignored the `userID` parameter, allowing any authenticated user to revoke any other user's session
- **High**: JWT access token was stored in `localStorage` (XSS-accessible); moved to in-memory module variable
- **High**: WebSocket connections sent the JWT as a URL query parameter (visible in server logs); authentication now uses the HttpOnly `access_token` cookie set during the HTTP upgrade
- **Medium**: Password reset URL included the user's email address as a query parameter; removed
- **Medium**: Email configuration in `server.go` was hardcoded to `"log"` provider and `noreply@tpt.local`, ignoring all `EMAIL_*` environment variables
- **Low**: Dead-code double base64 decode in `packages/auth/auth.go` `Compare` method removed
- **Low**: `UploadChunk` read the request body without a size cap; capped with `io.LimitReader` at the declared file size

### Security

- HttpOnly + SameSite=Strict cookies for access and refresh tokens
- `LIVE_HOOK_SECRET` production guard added alongside existing `JWT_SECRET` guard
- `UploadChunk` body capped at declared session size to prevent memory exhaustion

---

## [Unreleased]

- OAuth 2.0 login (Google, GitHub, Microsoft) — endpoint returns HTTP 501 until a provider library is wired
- Meilisearch search provider (PostgreSQL FTS is the default)
- ActivityPub / federation (future)
