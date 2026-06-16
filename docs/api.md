# API Reference

Base URL: `https://your-domain.com/api/v1`

All requests and responses use JSON (`Content-Type: application/json`). Authenticated endpoints require an `Authorization: Bearer <access_token>` header.

---

## Authentication

Access tokens expire after 15 minutes. Use the refresh endpoint to obtain a new token without requiring the user to log in again.

### Register

```
POST /auth/register
```

**Body**

```json
{
  "email": "user@example.com",
  "password": "strongpassword",
  "display_name": "My Name"
}
```

**Response** `201 Created`

```json
{ "user_id": "uuid", "email": "user@example.com", "display_name": "My Name" }
```

---

### Login

```
POST /auth/login
```

**Body**

```json
{ "email": "user@example.com", "password": "strongpassword" }
```

**Response** `200 OK`

```json
{
  "access_token": "eyJ...",
  "refresh_token": "opaque-token",
  "expires_in": 900
}
```

---

### Refresh access token

```
POST /auth/refresh
```

**Body**

```json
{ "refresh_token": "opaque-token" }
```

**Response** `200 OK` — same shape as login response with new tokens.

Refresh tokens rotate on every use. A reused token immediately invalidates the entire token family (all sessions for that user).

---

### Logout

```
POST /auth/logout
```

**Body**

```json
{ "refresh_token": "opaque-token" }
```

**Response** `204 No Content`

---

### Get current user

```
GET /auth/me
```

**Auth** Required

**Response** `200 OK`

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "display_name": "My Name",
  "role": "user",
  "avatar_url": "https://...",
  "bio": "About me"
}
```

---

### Update profile

```
PATCH /auth/me/profile
```

**Auth** Required

**Body** (all fields optional)

```json
{
  "display_name": "New Name",
  "bio": "Updated bio"
}
```

**Response** `200 OK` — updated user object.

---

### Upload avatar / banner

```
POST /auth/me/avatar
POST /auth/me/banner
```

**Auth** Required  
**Content-Type** `multipart/form-data`  
**Field** `file` — JPEG or PNG, max 5 MB.

**Response** `200 OK`

```json
{ "url": "https://..." }
```

---

### Password management

```
POST /auth/forgot-password
```

**Body** `{ "email": "user@example.com" }`  
**Response** `204 No Content` (always, to prevent user enumeration)

```
POST /auth/reset-password
```

**Body** `{ "token": "reset-token-from-email", "password": "newpassword" }`  
**Response** `204 No Content`

```
POST /auth/change-password
```

**Auth** Required  
**Body** `{ "current_password": "old", "new_password": "new" }`  
**Response** `204 No Content`

---

### Sessions

```
GET  /auth/sessions
DELETE /auth/sessions/{sessionID}
```

**Auth** Required.  `GET` lists active sessions; `DELETE` revokes a specific one.

---

## Videos (VOD)

### Get video

```
GET /videos/{videoID}
```

**Response** `200 OK`

```json
{
  "id": "uuid",
  "title": "My Video",
  "description": "...",
  "status": "ready",
  "visibility": "public",
  "duration_seconds": 120,
  "thumbnail_url": "https://...",
  "owner": { "id": "uuid", "display_name": "Creator" },
  "renditions": [
    { "label": "1080p", "manifest_url": "https://..." },
    { "label": "720p",  "manifest_url": "https://..." }
  ],
  "view_count": 42,
  "like_count": 7,
  "created_at": "2024-01-01T00:00:00Z"
}
```

**Video status values:** `uploading`, `processing`, `ready`, `error`  
**Visibility values:** `public`, `unlisted`, `private`

---

### Update video

```
PATCH /videos/{videoID}
```

**Auth** Required (owner or admin)

**Body** (all fields optional)

```json
{
  "title": "Updated Title",
  "description": "New description",
  "visibility": "unlisted"
}
```

---

### Delete video

```
DELETE /videos/{videoID}
```

**Auth** Required (owner or admin)  
**Response** `204 No Content`

---

### Get signed playback URLs

```
GET /videos/{videoID}/signed-urls
```

Returns pre-signed URLs valid for 1 hour. Used by the player for private/unlisted videos.

---

### Related videos

```
GET /videos/{videoID}/related
```

Returns up to 10 related public videos.

---

### Like / unlike

```
POST   /videos/{videoID}/like
DELETE /videos/{videoID}/like
GET    /videos/{videoID}/like
```

**Auth** Required for POST/DELETE. GET returns `{ "liked": true/false }`.

---

### Report a video

```
POST /videos/{videoID}/report
```

**Body**

```json
{ "reason": "spam", "details": "Optional explanation" }
```

**Reason values:** `spam`, `violence`, `hate_speech`, `copyright`, `misinformation`, `other`

---

## Upload (resumable)

Large files should be uploaded in chunks. The upload session tracks progress and allows resuming after interruption.

### Create upload session

```
POST /upload
```

**Auth** Required

**Body**

```json
{
  "filename": "myvideo.mp4",
  "size_bytes": 524288000,
  "content_type": "video/mp4",
  "title": "My Video",
  "description": "Optional description"
}
```

**Response** `201 Created`

```json
{
  "session_id": "uuid",
  "chunk_size_bytes": 10485760,
  "video_id": "uuid"
}
```

---

### Upload a chunk

```
POST /upload/{sessionID}/chunk
```

**Content-Type** `application/octet-stream`  
**Header** `Content-Range: bytes 0-10485759/524288000`

**Response** `200 OK`

```json
{ "received_bytes": 10485760 }
```

---

### Complete upload

```
POST /upload/{sessionID}/complete
```

Triggers the transcoding pipeline. The video status transitions to `processing`.

**Response** `200 OK` `{ "video_id": "uuid" }`

---

### Cancel upload

```
POST /upload/{sessionID}/cancel
```

**Response** `204 No Content`

---

### Get upload progress

```
GET /upload/{sessionID}
```

**Response** `200 OK`

```json
{
  "session_id": "uuid",
  "video_id": "uuid",
  "received_bytes": 10485760,
  "total_bytes": 524288000,
  "status": "uploading"
}
```

---

### List my upload sessions

```
GET /upload/sessions
```

---

## Comments

### List comments

```
GET /videos/{videoID}/comments?page=1&per_page=20
```

---

### Create comment

```
POST /videos/{videoID}/comments
```

**Auth** Required

**Body** `{ "body": "Great video!" }`

---

### Update / delete comment

```
PATCH  /comments/{commentID}
DELETE /comments/{commentID}
```

**Auth** Required (owner or admin/moderator)

---

### Like / unlike comment

```
POST   /comments/{commentID}/like
DELETE /comments/{commentID}/like
```

**Auth** Required

---

### Report comment

```
POST /comments/{commentID}/report
```

**Body** `{ "reason": "spam" }`

---

## Search

### Full-text search

```
GET /search?q=keyword&page=1&per_page=20
```

Returns public videos matching the query, ranked by relevance.

---

### Autocomplete

```
GET /search/autocomplete?q=key
```

Returns up to 10 title suggestions.

---

## Channels

### Get channel

```
GET /channels/{userID}
```

Returns the user's public profile.

---

### Channel videos

```
GET /channels/{userID}/videos?page=1&per_page=20
```

Returns the user's public videos.

---

### Channel live streams

```
GET /channels/{userID}/live
```

Returns the user's active live streams.

---

## Live streaming

### Create live stream

```
POST /live/streams
```

**Auth** Required

**Body**

```json
{
  "title": "My Stream",
  "description": "Optional description",
  "visibility": "public"
}
```

**Response** `201 Created`

```json
{
  "id": "uuid",
  "title": "My Stream",
  "stream_key": "sk_abc123...",
  "rtmp_url": "rtmp://your-domain.com/live",
  "status": "idle"
}
```

The `stream_key` is shown only once. Store it securely. Losing it requires deleting and recreating the stream.

---

### List my streams

```
GET /live/streams
```

---

### Get stream

```
GET /live/streams/{streamID}
```

**Response** includes `status` (`idle`, `live`, `ended`) and viewer-facing URLs.

---

### Update stream metadata

```
PATCH /live/streams/{streamID}
```

**Auth** Required (owner)

**Body** (all optional) `{ "title": "...", "description": "...", "visibility": "..." }`

---

### Delete stream

```
DELETE /live/streams/{streamID}
```

**Auth** Required (owner or admin)

---

### Currently live streams

```
GET /live/streams/live
```

Returns all currently broadcasting public streams.

---

### Stream playback URLs

```
GET /live/streams/{streamID}/urls
```

**Response**

```json
{
  "hls_url": "http://mediamtx:8888/live/{streamKey}/index.m3u8",
  "webrtc_url": "http://mediamtx:8889/live/{streamKey}/whep"
}
```

---

### DVR window info

```
GET /live/streams/{streamID}/dvr
```

Returns the available DVR window (default 15 minutes) for seeking in the current stream.

---

## Live chat

### WebSocket connection

```
GET /live/streams/{streamID}/chat/ws
```

Upgrade to WebSocket. Authentication is optional; unauthenticated viewers connect as guests.

**Incoming message types** (server → client):

| Type | Payload |
|------|---------|
| `message` | `{ id, user_id, display_name, body, timestamp }` |
| `delete` | `{ message_id }` |
| `timeout` | `{ user_id, expires_at }` |
| `ban` | `{ user_id }` |
| `lock` | `{}` |
| `unlock` | `{}` |

**Outgoing message** (client → server):

```json
{ "type": "message", "body": "Hello!" }
```

---

### Message history

```
GET /live/streams/{streamID}/chat/messages?limit=50&before=message_id
```

---

### Moderation (stream owner / moderator)

```
DELETE /live/streams/{streamID}/chat/messages/{messageID}
POST   /live/streams/{streamID}/chat/users/{userID}/timeout
DELETE /live/streams/{streamID}/chat/users/{userID}/timeout
POST   /live/streams/{streamID}/chat/users/{userID}/ban
DELETE /live/streams/{streamID}/chat/users/{userID}/ban
POST   /live/streams/{streamID}/chat/lock
DELETE /live/streams/{streamID}/chat/lock
```

**Timeout body** `{ "duration_seconds": 300 }`

---

## Reports & appeals

```
POST /reports
POST /users/{userID}/report
POST /live/streams/{streamID}/report
POST /reports/{reportID}/appeal
```

All report endpoints accept `{ "reason": "...", "details": "..." }`.  
`appeal` accepts `{ "statement": "Why the content should be restored" }`.

---

## Admin endpoints

All admin endpoints require `role = admin` or `role = moderator` (some require `admin` only — see below).

### Moderation dashboard

```
GET  /admin/moderation/stats
GET  /admin/reports?status=open&page=1
GET  /admin/reports/{reportID}
POST /admin/reports/{reportID}/assign        # body: { "moderator_id": "uuid" }
POST /admin/reports/{reportID}/unassign
POST /admin/reports/{reportID}/resolve       # body: { "action": "hide_content", "notes": "..." }
POST /admin/reports/{reportID}/dismiss       # body: { "notes": "..." }
PUT  /admin/reports/{reportID}/notes         # body: { "notes": "..." }
POST /admin/reports/{reportID}/appeal        # body: { "granted": true, "notes": "..." }
```

**Report status values:** `open`, `assigned`, `resolved`, `dismissed`

**Action values:** `hide_content`, `unpublish_video`, `delete_video`, `remove_comment`, `suspend_user`, `ban_user`, `terminate_live_stream`, `lock_live_chat`, `restore_content`

---

### Moderation actions

```
POST /admin/moderation/actions               # execute an action directly
GET  /admin/moderation/actions?page=1
GET  /admin/moderation/actions/{actionID}
POST /admin/moderation/actions/{actionID}/reverse
```

---

### Audit log

```
GET /admin/audit-log?page=1&per_page=50&actor_id=uuid&action=ban_user
```

---

### User management (admin only)

```
GET   /admin/users?page=1&search=term
PATCH /admin/users/{userID}                  # body: { "role": "moderator", "status": "active" }
```

**User status values:** `active`, `suspended`, `banned`

---

### Video management (admin only)

```
GET    /admin/videos?page=1&status=ready
PATCH  /admin/videos/{videoID}
DELETE /admin/videos/{videoID}
```

---

### Comment management (admin only)

```
GET   /admin/comments?page=1
PATCH /admin/comments/{commentID}
```

---

### System (admin only)

```
GET /admin/health
GET /admin/system/status
GET /admin/system/metrics       # Prometheus-format metrics
GET /admin/settings
```

---

## Health & diagnostics

```
GET /healthz    # liveness — returns 200 if the process is up
GET /readyz     # readiness — returns 200 only when DB and Redis are connected
GET /api/v1/ping
```

---

## Error responses

All errors return a consistent JSON envelope:

```json
{
  "error": "human-readable message",
  "code": "machine_readable_code"
}
```

| HTTP status | Meaning |
|-------------|---------|
| 400 | Bad request / validation error |
| 401 | Missing or invalid access token |
| 403 | Authenticated but not authorised |
| 404 | Resource not found |
| 409 | Conflict (e.g. email already registered) |
| 422 | Unprocessable entity |
| 429 | Rate limit exceeded |
| 500 | Internal server error |

---

## Rate limits

| Endpoint group | Limit |
|----------------|-------|
| Auth (login, register, password reset) | 10 req/min, burst 3 |
| Admin panel | 30 req/min |
| Everything else | 100 req/min, burst 10 |

When rate-limited the response includes `Retry-After` (seconds) in the header.
